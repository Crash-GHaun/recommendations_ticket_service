package userspacequeries

type QueryInfo struct {
	ResponseStruct	interface{}
	TicketTpl	string
}

var QueriesMap = map[string]QueryInfo{
	"diskType.tpl": {
		ResponseStruct: &struct {
			instance_id	string
			disk_size int
			storage_type string
			max_used_iops int
			current_value_limit int
			recommendation struct {
				name string
				iop_limit int
			}
		}{},
		TicketTpl: "Hello", 
	},
}