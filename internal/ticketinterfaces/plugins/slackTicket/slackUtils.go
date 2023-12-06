// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"

	b "ticketservice/internal/bigqueryfunctions"
	t "ticketservice/internal/ticketinterfaces"
	u "ticketservice/internal/utils"
)

func verifyRequestSignature(header http.Header, body []byte) bool {
    // Extract the signature and timestamp from the header
    signature := header.Get("X-Slack-Signature")
    timestamp := header.Get("X-Slack-Request-Timestamp")
    // Ensure the timestamp is not too old
    timestampInt, err := strconv.Atoi(timestamp)
    if err != nil {
		u.LogPrint(2, "Verify Request Signature Failed at strconv.Atoi: %v", err)
        return false
    }
    age := time.Now().Unix() - int64(timestampInt)
    if age > 300 {
        return false
    }

    // Concatenate the timestamp and request body
    sigBaseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
    // Hash the base string with the Slack signing secret
    signatureHash := hmac.New(sha256.New, []byte(slackSigningSecret))
    signatureHash.Write([]byte(sigBaseString))
    expectedSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(signatureHash.Sum(nil)))

    // Compare the expected signature to the actual signature
	equal := hmac.Equal([]byte(signature), []byte(expectedSignature))
	u.LogPrint(1, "Received Sig: %s   Calculated Sig: %s", signature, expectedSignature)
    return equal
}

func (s *SlackTicketService) createNewChannel(channelName string) (*slack.Channel, error){
	// Lock the cache because there could be multiple 
	// goroutines checking channels
	// and if we create a channel, we don't want to try 
	// creating multiple.
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	channel, exists := s.channelCache[channelName]
	if exists {
		// The channel exists, simply return the channel
		return &channel, nil
	}
	// Create channel if it doesn't exist
	newChannel, err := s.slackClient.CreateConversation(slack.CreateConversationParams{
		ChannelName: channelName,
	})
	if err != nil {
		// If channel name exists we need to update the cache.
		if strings.Contains(err.Error(), "name_taken") || strings.Contains(err.Error(), "channel already exists") {

			err = s.updateChannelCache(false)// False lock because it's already locked
			if err != nil {
				// Many more errors may come after this because something
				// went terribly wrong, and now the cache is cleared
				u.LogPrint(3, "Error creating channel cache: %s", err)
				return nil, err
			}
			// Use recursion since we need to confirm it's created?
			s.cacheMutex.Unlock()
			return s.createNewChannel(channelName)
		} else {
			return nil, err
		}
	}
	// Add channel to cache
	s.channelCache[newChannel.Name] = *newChannel
	return newChannel, nil
}


func (s *SlackTicketService) createChannelAsTicket(ticket *t.Ticket, row t.RecommendationQueryResult) (string, error) {
	lastSlashIndex := strings.LastIndex(row.TargetResource, "/")
	secondToLast := strings.LastIndex(row.TargetResource[:lastSlashIndex], "/")
	// This could be moved to BQ Query. But ehh
	ticket.CreationDate = time.Now()
	ticket.LastUpdateDate = time.Now()
	ticket.LastPingDate = time.Now()
	ticket.SnoozeDate = time.Now().AddDate(0,0,7)
	ticket.Subject = fmt.Sprintf("%s-%s",
			row.Recommender_subtype,
			nonAlphanumericRegex.ReplaceAllString(
				row.TargetResource[secondToLast+1:],
				""))
	ticket.RecommenderID = row.Recommender_name
	
	// Create Ticket Title
	var titleBuffer bytes.Buffer
	err := s.titleTemplate.Execute(&titleBuffer, map[string]interface{}{"Row": row, "Ticket": ticket})
	if err != nil {
		u.LogPrint(3,"Error Executing Channel Name Template")
		return "", err
	}
	channelName := titleBuffer.String()
	channelName = strings.ReplaceAll(channelName, " ", "")
	// According to this document the string length can be a max of 80
	// https://api.slack.com/methods/conversations.create
	sliceLength := 80
	stringLength := len(channelName) - 1
	if stringLength  < sliceLength {
		sliceLength = stringLength
	}
	channelName = strings.ToLower(channelName[0:sliceLength])
	u.LogPrint(1,"Creating Channel: "+channelName)
	channel, err := s.createNewChannel(channelName)
	if err != nil {
		u.LogPrint(3,"Error creating channel")
		return "", err
	}

	ticket.IssueKey = channel.ID
	_, err = s.slackClient.InviteUsersToConversation(channel.ID, ticket.Assignee...)
	if err != nil {
		// If user is already in channel we should continue
		if err.Error() != "already_in_channel" {
			u.LogPrint(3,"Failed to invite users to channel:")
			return channel.ID, err
		}
		u.LogPrint(1,"User(s) were already in channel")
	}

	// Ping Channel with details of the Recommendation
	s.UpdateTicket(ticket, row)
	u.LogPrint(2,"Created Channel: "+channelName+"   with ID: "+channel.ID)
	return channel.ID, nil
}

func (s *SlackTicketService) createThreadAsTicket(ticket *t.Ticket, row t.RecommendationQueryResult) (string, error) {
	u.LogPrint(1, "Creating Thread As Ticket")
	lastSlashIndex := strings.LastIndex(row.TargetResource, "/")
	secondToLast := strings.LastIndex(row.TargetResource[:lastSlashIndex], "/")
	// This could be moved to BQ Query. But ehh
	ticket.CreationDate = time.Now()
	ticket.LastUpdateDate = time.Now()
	ticket.LastPingDate = time.Now()
	ticket.SnoozeDate = time.Now().AddDate(0,0,7)
	ticket.Subject = fmt.Sprintf("%s-%s-%s",
			row.Project_name,
			nonAlphanumericRegex.ReplaceAllString(
				row.TargetResource[secondToLast+1:],
				""),
			row.Recommender_subtype)
	ticket.RecommenderID = row.Recommender_name
	channelName := strings.ToLower(ticket.TargetContact)

	// Replace multiple characters using regex to conform to Slack channel name restrictions
	re := regexp.MustCompile(`[\s@#._/:\\*?"<>|]+`)
	channelName = re.ReplaceAllString(channelName, "-")

	u.LogPrint(1, "Creating Channel: "+channelName)
	channel, err := s.createNewChannel(channelName)
	if err != nil {
		u.LogPrint(3, "Error creating channel for thread as ticket: %s\n", err)
		return "", err
	}
	// Invite users to the channel
	_, err = s.slackClient.InviteUsersToConversation(channel.ID, ticket.Assignee...)
	if err != nil {
		// If user is already in channel we should continue
		if err.Error() != "already_in_channel" {
			u.LogPrint(3,"Failed to invite users to channel: %s\n", err)
			return channel.ID, err
		}
		u.LogPrint(1,"User(s) were already in channel")
	}

	// Send message to the created channel to create "ticket/thread"
	messageOptions := slack.MsgOptionText(ticket.Subject, false)
	_ ,timestamp, err := s.slackClient.PostMessage(channel.ID, messageOptions)
	if err != nil {
		u.LogPrint(3, "Failed to send message to channel")
		return channel.ID, err
	}

	ticket.IssueKey = channel.ID + "-" + timestamp

	s.UpdateTicket(ticket, row)
	u.LogPrint(1, "Created Ticket in Channel: "+channelName+" with ID: "+timestamp)
	return ticket.IssueKey, nil
}

// C = Channel, t = ThreadTimeStamp, m = message you want to send
func (s *SlackTicketService) sendSlackMessage(c string, t string, m string) error{
	// Send the message to the channel in which the event occurred
	u.LogPrint(1, "Sending message to channel: %s, timestamp: %s, with message: %s", c,t,m)
	message := slack.MsgOptionText(m, false)
	if !s.channelAsTicket {
		_, _, _, err := s.slackClient.SendMessage(c, slack.MsgOptionTS(t), message)
		if err != nil {
			u.LogPrint(3, "Failed to respond in thread: %v", err)
			return err
		}
		return nil
	}
	_, _, err := s.slackClient.PostMessage(c, message)
	if err != nil {
		u.LogPrint(3,"Error sending message: %s\n", err)
		return err
	}
	return nil
}

func (s *SlackTicketService) parseAndGetTicket(channel, timestamp string) (t.Ticket, error) {
	issueKey := channel
	if !s.channelAsTicket {
		issueKey = fmt.Sprintf("%v-%v", channel, timestamp)
	}
	ticket, err := b.GetTicketByIssueKey(issueKey)
	if err != nil {
		u.LogPrint(3, "[SLACK] Error getting ticket from Bigquery: %v", err)
		return t.Ticket{}, err
	}
	return *ticket, nil
}
