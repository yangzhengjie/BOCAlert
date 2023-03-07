
>更新日志:

20230217 初始版本

20230227
1. 原SystemName显示TiDB集群名称，现在SystemName 统一改为C-TIQ,C-PSI,C-CBD
2. 磁盘告警，需要增加告警磁盘的挂载点,涉及到的告警有:NODE_disk_used_more_than_80%,NODE_disk_used_more_than_90%,NODE_disk_used_more_than_95%
3. 调整告警转发程序打印日志的内容，打印完整的告警信息。
>运行方式:

` 
go run BOC_alert.go  -host 172.16.6.194 -port 8899 -system C-TIQ
go run BOC_alert.go  -host 172.16.6.194 -port 8899 -system C-PSI
go run BOC_alert.go  -host 172.16.6.194 -port 8899 -system C-CBD
`
