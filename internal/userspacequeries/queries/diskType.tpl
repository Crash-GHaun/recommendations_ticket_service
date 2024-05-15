with disk_view as  (SELECT 
  kind, 
  m.name, 
  type, 
  value_type,
  JSON_EXTRACT_SCALAR(m.resource.labels, "$.instance_id") as instance_id,
  LOWER(JSON_EXTRACT_SCALAR(m.labels, "$.storage_type")) as storage_type,
  max(value.int64_value) as max_value,

  #a.resource.data,
  #Currently only does the first disk. I need to add more disks and see what's up.

  PARSE_NUMERIC(JSON_EXTRACT_SCALAR(JSON_EXTRACT_ARRAY(a.resource.data, "$.disks")[0], "$.diskSizeGb")) as disk_size,
 
  #Will need to make this adjustable and configurable to a timeframe. 

  min(start_time),
  max(end_time),
  #Currently only focusing on PD Standard as I need to add additional disks to check how it will show. 
  
  JSON_EXTRACT_SCALAR(JSON_EXTRACT_ARRAY(a.resource.data, "$.disks")[0], "$.type") as disk_type,

  FROM `{{ .MetricsTable }}` as m
  left join `{{ .AssetTable}}` as a
  on STRING(JSON_EXTRACT(m.resource.labels, "$.instance_id")) = JSON_EXTRACT_SCALAR(
            a.resource.data, "$.id")
  where
  a.name is not null and
  m.type like "%disk%ops%" and
  value.int64_value is not null
  and kind = "GAUGE"
  group by kind, m.name, type, value_type, instance_id, storage_type, disk_size, disk_type
),
limit_table as (
Select *,

case disk_type
  when "PERSISTENT" then
    case 
      when (storage_type="pd-standard" and type like "%read%") then (disk_size * 0.75)
      when (storage_type="pd-standard" and type like "%write%") then (disk_size * 1.5)
      when (storage_type="pd-balanced") then (disk_size * 6)
      when (storage_type="pd-ssd") then (disk_size * 30)
    end
    
  #Add 3000 because: https://cloud.google.com/compute/docs/disks/performance#baseline_performance
end + 3000 as current_value_limit,
[
  STRUCT(if(type like "%write%", (disk_size * 1.5 + 3000), (disk_size * 1.5 + 3000)) as iop_limit, "pd-standard" as name),
  STRUCT((disk_size * 6 + 3000) as iop_limit, "pd-balanced" as name),
  STRUCT((disk_size * 30 + 3000) as iop_limit, "pd-ssd" as name)
  ] as iop_limits,

from disk_view
),
compare_view as (
Select *,
(SELECT as STRUCT name, iop_limit FROM UNNEST(iop_limits) WHERE iop_limit >= max_value order by iop_limit limit 1) as recommendation,
from limit_table )

select 
instance_id,
disk_size,
storage_type,
max(max_value) as max_used_iops,
current_value_limit,
ANY_VALUE(recommendation) as recommendation
from compare_view where storage_type != recommendation.name
group by all