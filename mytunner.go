package main

import (
"database/sql"
"flag"
"fmt"
_ "github.com/go-sql-driver/mysql"
"os"
"strconv"
"strings"
)

const (
	//定义每分钟的秒数
	SecondsPerMinute = 60
	//定义每小时的秒数
	SecondsPerHour = SecondsPerMinute * 60
	//定义每天的秒数
	SecondsPerDay = SecondsPerHour * 24
)
type mysqld struct {
	host string
	port int
	user string
	passwd string
}

var m = mysqld{
	host: "127.0.0.1",
	port: 3306,
	user: "test",
	passwd: "test",
}

func Str2Int(constr string) int {
	conint, err := strconv.Atoi(constr)
	if err != nil {
		return 0
	}
	return conint
}

func Str2Int64(str string) int64 {
	i, err := strconv.ParseInt(str,10,64)
	if err != nil {
		return 0
	}
	return i
}
func resolveTime(seconds int64) string {
	//每天秒数
	day:= seconds / SecondsPerDay
	//小时
	hour := (seconds - day * SecondsPerDay) / SecondsPerHour
	//分钟
	minute := (seconds - day  * SecondsPerDay - hour * SecondsPerHour) / SecondsPerMinute
	return fmt.Sprintf("%d days %d hour %d minute",day, hour, minute)
}

func typeChek(i interface{}) (int){
	switch i.(type) {
	case nil:
		return 0
	case int:
		return 1
	case string:
		return 2
	case float64:
		return 3
	default:
		return 4
	}
}

func dbStatusCheck(dataUrl string)  {
	var variableSql = "SHOW VARIABLES"
	var statusSql = "SHOW /*!50002 GLOBAL */ STATUS"
	db, err := sql.Open("mysql",dataUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()

	// 存放计算结果
	var result map[string]int64
	//mycal = make(map[string]interface{})
	result = make(map[string]int64)

	// 存放变量及检查结果
	mystat := make(map[string]string)


	rows, err := db.Query(variableSql)
	if err != nil {
		fmt.Printf("show variables exec failed , %v\n",err)
		return
	}

	var variable string
	var val string
	for rows.Next() {
		rows.Scan(&variable,&val)
		//fmt.Printf("%s %s\n",variable,val)
		mystat[strings.ToLower(variable)] = val
	}

	rows, err = db.Query(statusSql)
	if err != nil {
		fmt.Printf("show status exec failed , %v\n",err)
		return
	}
	for rows.Next() {
		rows.Scan(&variable,&val)
		//fmt.Printf("%s %s\n",variable,val)
		mystat[strings.ToLower(variable)] = val
	}

	result["connections"] = Str2Int64(mystat["connections"])
	result["max_connections"] = Str2Int64(mystat["max_connections"])
	result["max_used_connections"] = Str2Int64(mystat["max_used_connections"])
	result["threads_running"] = Str2Int64(mystat["threads_running"])

	result["tps"] = (Str2Int64(mystat["com_commit"]) + Str2Int64(mystat["com_rollback"])) / Str2Int64(mystat["uptime"])
	result["qps"] = Str2Int64(mystat["questions"])/Str2Int64(mystat["uptime"])
	//result["uptime"] = resolveTime(Str2Int64(mystat["uptime"]))
	uptimeStr := resolveTime(Str2Int64(mystat["uptime"]))

	//缓存最大使用情况
	result["per_thread_buffers"] = Str2Int64(mystat["read_buffer_size"]) + Str2Int64(mystat["read_rnd_buffer_size"]) + Str2Int64(mystat["sort_buffer_size"]) + Str2Int64(mystat["thread_stack"]) + Str2Int64(mystat["join_buffer_size"])
	result["total_per_thread_buffers"] = Str2Int64(mystat["per_thread_buffers"]) * Str2Int64(mystat["max_connections"])
	result["max_total_per_thread_buffers"] = result["per_thread_buffers"] * Str2Int64(mystat["max_used_connections"])

	if Str2Int64(mystat["tmp_table_size"]) > Str2Int64(mystat["max_heap_table_size"]) {
		result["max_tmp_table_size"] = Str2Int64(mystat["tmp_table_size"])
	} else {
		result["max_tmp_table_size"] = Str2Int64(mystat["max_heap_table_size"])
	}

	result["server_buffers"] = Str2Int64(mystat["key_buffer_size"]) + result["max_tmp_table_size"] + Str2Int64(mystat["innodb_buffer_pool_size"]) + Str2Int64(mystat["innodb_additional_mem_pool_size"]) + Str2Int64(mystat["innodb_log_buffer_size"]) + Str2Int64(mystat["query_cache_size"])

	result["max_used_memory"] = result["server_buffers"] + result["max_total_per_thread_buffers"]
	result["total_possible_used_memory"] = result["server_buffers"] + result["total_per_thread_buffers"]

	max_con_rate := Str2Int64(mystat["max_used_connections"]) * 100 / result["max_connections"]
	if max_con_rate > 100 {
		result["max_con_use_pct"] = 100
	} else {
		result["max_con_use_pct"] = max_con_rate
	}

	read := Str2Int64(mystat["com_select"])
	write := Str2Int64(mystat["com_delete"]) + Str2Int64(mystat["com_insert"]) + Str2Int64(mystat["com_update"]) + Str2Int64(mystat["com_replace"])
	result["read_pct"] = (read * 100)/(read + write)

	result["slow_queries"] = Str2Int64(mystat["slow_queries"])
	result["questions"] = Str2Int64(mystat["questions"])
	result["pct_slow_queries"] = result["slow_queries"] * 100 / (1 + result["questions"]) + 1

	result["innodb_buffer_pool_size"] = Str2Int64(mystat["innodb_buffer_pool_size"]) / 1024 * 1024 * 1024
	result["innodb_buffer_read_hit"] = Str2Int64(mystat["innodb_buffer_pool_reads"]) * 100 / (1 + Str2Int64(mystat["innodb_buffer_pool_read_requests"]))
	result["innodb_buffer_write_hit"] = Str2Int64(mystat["innodb_buffer_pool_wait_free"]) * 100 / (1 + Str2Int64(mystat["Innodb_buffer_pool_write_requests"]))

	//connect aborted ratio
	result["aborted_con_pct"] = Str2Int64(mystat["aborted_connects"]) * 100 / result["connections"]

	//mysql network traffic
	result["bytes_received"] = 1 + Str2Int64(mystat["bytes_received"]) / 1024 / 1024 / 1024
	result["bytes_sent"] = 1 + Str2Int64(mystat["bytes_sent"]) / 1024 / 1024 / 1024

	// disk temp tables
	result["created_tmp_tables"] = Str2Int64(mystat["created_tmp_tables"])
	result["created_tmp_disk_tables"] = Str2Int64(mystat["created_tmp_disk_tables"])
	result["pct_temp_disk"] = Str2Int64(mystat["created_tmp_disk_tables"]) * 100 / (Str2Int64(mystat["created_tmp_tables"]) + 1)
	//线程缓存命中率
	result["threads_created"] = Str2Int64(mystat["threads_created"])
	result["thread_cache_hit_rate"] = 100 - result["threads_created"] * 100 / result["connections"]

	//表缓存命中率
	result["table_cache_hit_rate"] = Str2Int64(mystat["table_cache_hit_rate"])
	result["open_tables"] = Str2Int64(mystat["open_tables"])
	result["opened_tables"] = Str2Int64(mystat["opened_tables"])
	result["table_cache_hit_rate"] = Str2Int64(mystat["table_open_cache_hits"]) * 100 / (1 + Str2Int64(mystat["table_open_cache_hits"]) + Str2Int64(mystat["table_open_cache_misses"]))

	//磁盘文件排序比例
	result["total_sorts"] = Str2Int64(mystat["sort_scan"]) + Str2Int64(mystat["sort_range"])

	//加1避免除数为0
	result["sort_merge_passes"] = Str2Int64(mystat["sort_merge_passes"])
	result["pct_temp_sort_table"] = Str2Int64(mystat["sort_merge_passes"]) * 100 / (1 + Str2Int64(mystat["sort_scan"]) + Str2Int64(mystat["sort_range"]))

	// Joins未使用索引比例
	//result["joins_without_indexes"] = Str2Int64(mystat["select_range_check"]) + Str2Int64(mystat["select_full_join"])

	//表立刻获得锁比例
	result["table_locks_immediate"] = Str2Int64(mystat["table_locks_immediate"])
	result["total_table_locks"] = Str2Int64(mystat["table_locks_waited"]) + Str2Int64(mystat["table_locks_immediate"])
	if result["total_table_locks"] != 0 {
		result["pct_table_locks_immediate"] = result["table_locks_immediate"] * 100 / (Str2Int64(mystat["table_locks_waited"]) + result["table_locks_immediate"])
	} else {
		result["pct_table_locks_immediate"] = 0
	}

	//# open file limit used
	result["open_files_limit"] = Str2Int64(mystat["open_files_limit"])
	result["open_files"] = Str2Int64(mystat["open_files"])
	result["pct_files_open"] = result["open_files"] * 100 / result["open_files_limit"]

	if Str2Int64(mystat["innodb_buffer_pool_read_requests"]) != 0 {
		result["innodb_buffer_pool_hit"] = 100 - 100 * Str2Int64(mystat["innodb_buffer_pool_reads"]) / Str2Int64(mystat["innodb_buffer_pool_read_requests"])
	} else {
		result["innodb_buffer_pool_hit"] = 0
	}


	//fmt.Println("mysql runtime:",uptimeStr)
	fmt.Println("------------- MySQL Performance Metrics -------------------------------------")
	fmt.Printf("       mysql runtime : %s\n" , uptimeStr)
	fmt.Printf("      Running threads: %d    TPS: %d QPS: %d\n" , result["threads_running"], result["tps"],result["qps"])
	fmt.Printf("   Read / Write Ratio: %d%% / %d%%\n" , result["read_pct"], 100-result["read_pct"])
	fmt.Printf("      used max memory: %d MB\n"  , result["max_total_per_thread_buffers"]/1024/1024)
	fmt.Printf("Max possible use  mem: %d MB \n" ,result["total_possible_used_memory"]/1024/1024)
	fmt.Printf("   Slow queries Ratio: %d%% (%d/%d)\n" ,result["pct_slow_queries"], result["slow_queries"], result["questions"])
	fmt.Printf("Max usable connection: %d%% (%d/%d)\n",result["pct_connections_used"], result["max_used_connections"], result["max_connections"])
	fmt.Printf("Sort temporary tables: %d%% (%d temp sorts / %d sorts)\n" , result["pct_temp_sort_table"], result["sort_merge_passes"], result["total_sorts"])
	fmt.Printf("Disk temporary tables: %d%% (%d on disk / %d total)\n" ,result["pct_temp_disk"], result["created_tmp_disk_tables"], result["created_tmp_tables"])
	fmt.Printf("Thread cache hit rate: %d%% (%d created / %d connections)\n",result["thread_cache_hit_rate"], result["threads_created"], result["connections"])
	fmt.Printf(" Table cache hit rate: %d%% (%d open / %d opened)\n",result["table_cache_hit_rate"], result["open_tables"], result["opened_tables"])
	fmt.Printf(" Open file limit used: %d%% (%d/%d)\n" ,result["pct_files_open"], result["open_files"], result["open_files_limit"])
	fmt.Printf(" Table locks acquired: %d%% (%d immediate / %d locks)\n", result["pct_table_locks_immediate"], result["table_locks_immediate"], result["total_table_locks"])
	fmt.Printf("     InnoDB log waits: %d\n",result["innodb_log_waits"])
	fmt.Printf(" InnoDB buff hit rate: %d%%\n" , result["innodb_buffer_pool_hit"])
}

func engineCheck(dbUrl string) {
	db, err := sql.Open("mysql",dbur)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()

	engine := "select engine,count(*) num from information_Schema.tables where table_schema not in('mysql','information_schema','performance_schema','sys') and engine is not null group by engine;"
	rows, err := db.Query(engine)
	if err != nil {
		fmt.Printf("show variables exec failed , %v\n",err)
		return
	}
	var variable string = ""
	var val int = 0
	fmt.Println("------------------------------mysql table engine information------------------------------")
	fmt.Printf("%40s%40s\n","engine","num")
	for rows.Next() {
		rows.Scan(&variable,&val)
		fmt.Printf("%40s%40d\n",variable,val)
	}

	noInnodb := "select table_schema,table_name,engine from information_schema.tables where engine not in('innodb') and table_type='base table' and table_schema  not in('mysql','information_schema','performance_schema','sys')"
	rows, err = db.Query(noInnodb)
	if err != nil {
		fmt.Printf("show variables exec failed , %v\n",err)
		return
	}
	var ts,tn,en string = "","",""
	fmt.Println("------------------------------mysql not innodb engine table information-------------------")
	fmt.Printf("%40s%40s%40s\n","table_schema","table_name","engine")
	for rows.Next() {
		rows.Scan(&ts,&tn,&en)
		fmt.Printf("%40s%40s%40s\n",ts,tn,en)
	}
}

func charsetCheck(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()
	character_sql := "select table_schema,table_name,COLUMN_NAME,DATA_TYPE,CHARACTER_SET_NAME,COLLATION_NAME from information_Schema.columns " +
		"where CHARACTER_SET_NAME not in('utf8','utf8mb4') and COLLATION_NAME not in('utf8mb4_bin','utf8_bin','utf8_general_ci','utf8mb4_general_ci') " +
		"and table_schema not in('mysql','information_schema','performance_schema','sys')"
	rows, err := db.Query(character_sql)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var schema,table,column,data_type,charact,collation string = "","","","","",""
	fmt.Println("----------Non-utf8[db.tablename](column_Name type character collation) information----------")
	for rows.Next() {
		rows.Scan(&schema,&table,&column,&data_type,&charact,&collation)
		fmt.Printf("%s.%s(%s %s %s %s)\n",schema,table,column,data_type,charact,collation)
	}
}
func dbSize(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()
	dbSize := "select substring_index(name,'/',1) db,truncate(sum(FILE_SIZE)/1024/1024,0) db_size  from information_schema.INNODB_SYS_TABLESPACES " +
		"where name not like 'mysql%' and name not like 'information_schema%' and name not like 'performance_schema%' and " +
		"name not like 'sys%' and FILE_SIZE > 1024 * 1024 * 1024 group by db order by db_size desc;"
	rows, err := db.Query(dbSize)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var dbname string
	var size int
	fmt.Println("--------------------db disk size > 1G information--------------------")
	fmt.Printf("%40s%40s\n","database","size(MB)")
	for rows.Next() {
		rows.Scan(&dbname, &size)
		fmt.Printf("%40s%40d\n",dbname,size)
	}

}
func tableSize(dbUrl string){
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	tableSize57 := "select name,truncate(file_size/1024/1024,0) size from information_schema.INNODB_SYS_TABLESPACES where name not like 'mysql%' and " +
		" name not like 'information_schema%' and name not like 'performance_schema%' and name not like 'sys%' and file_size>1024*1024*1024"
	rows, err := db.Query(tableSize57)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	table := ""
	size := 0
	fmt.Println("----------table size > 1024M information----------")
	fmt.Printf("%40s%40s\n","dbname/table","size(MB)")
	for rows.Next() {
		rows.Scan(&table, &size)
		fmt.Printf("%40s%40d\n",table,size)
	}
}

func optimizeTab(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()

	optimizeTab := "select b.db,b.table_name,b.disk_table_size,a.table_size from (SELECT table_schema,table_name,sum(data_length+index_length)/1024/1024/1024 table_size FROM " +
		"information_schema.tables where  data_length>1*1024*1024*1024 group by table_name ) a join  (select substring_index(name,'/',1) db," +
		"substring_index(substring_index(name,'/',-1),'#',1) table_name,sum(file_size)/1024/1024/1024 disk_table_size  from information_schema.INNODB_SYS_TABLESPACES " +
		"where file_size>1*1024*1024*1024 group by table_name) b on a.table_Schema=b.db and a.table_name=b.table_name and abs(a.table_size-b.disk_table_size)/table_size>0.2;"
	rows, err := db.Query(optimizeTab)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	schema := ""
	table  := ""
	var table_size,disk_table_size int = 0,0
	fmt.Println("----------Table size difference exceeds 20% && size > 1G ----------")
	for rows.Next() {
		rows.Scan(&schema,&table,&table_size,&disk_table_size)
		fmt.Printf("%s.%s:(schema_size) %dG , disk file size: %d G \n",schema,table,table_size,disk_table_size)
	}
}

func noPrimaryKey(dbUrl string) {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	noPrimaryKey := "select t1.table_schema,t1.table_name from information_schema.tables t1 left join information_schema.table_constraints t2 on t1.table_schema = t2.table_schema " +
		"and t1.table_name = t2.table_name and t2.constraint_name ='primary' where t2.table_name is null and t1.table_schema not in ('mysql','information_schema','performance_schema','sys') " +
		"and table_type='base table' order by t1.table_schema,t1.table_name;"
	rows, err := db.Query(noPrimaryKey)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	schema := ""
	table := ""
	fmt.Println("----------No primary key table(db.table) information ----------")
	//fmt.Println("database.table_name")
	for rows.Next() {
		rows.Scan(&schema,&table)
		fmt.Printf("%s.%s\n",schema,table)
	}
}

func redundantIndex(dbUrl string) {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()
	redundantIndex := "select table_schema,table_name,redundant_index_name,redundant_index_non_unique,dominant_index_name,sql_drop_index from sys.schema_redundant_indexes"
	rows, err := db.Query(redundantIndex)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var schema,table,index_name,unique,dominant,sql string = "","","","","",""
	fmt.Println("----------Redundant index table information ----------")
	fmt.Println("table_schema | table_name | redundant_index_name | redundant_index_non_unique | dominant_index_name | sql_drop_index")
	for rows.Next() {
		rows.Scan(&schema,&table,&index_name,&unique,&dominant,&sql)
		fmt.Printf("%s | %s | %s | %s | %s | %s \n",schema,table,index_name,unique,dominant,sql)
	}
}

func unUseIndex(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()
	//unUseIndex56 := "select `performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_SCHEMA` AS `object_schema`,`performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_NAME` AS `object_name`," +
	//	"`performance_schema`.`table_io_waits_summary_by_index_usage`.`INDEX_NAME` AS `index_name` from `performance_schema`.`table_io_waits_summary_by_index_usage` " +
	//	"where ((`performance_schema`.`table_io_waits_summary_by_index_usage`.`INDEX_NAME` is not null) and (`performance_schema`.`table_io_waits_summary_by_index_usage`.`COUNT_STAR` = 0) " +
	//	"and (`performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_SCHEMA` not in("mysql","information_schema","sys","performance_schema")) and (`performance_schema`.`table_io_waits_summary_by_index_usage`.`INDEX_NAME` <> "PRIMARY")) " +
	//	"order by `performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_SCHEMA`,`performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_NAME`"
	unUseIndex57 := "select object_schema,object_name,index_name from sys.schema_unused_indexes"
	rows, err := db.Query(unUseIndex57)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var schema,table,index string = "","",""
	fmt.Println("----------Unused index table information----------")
	fmt.Println("database.table_name\t:\tindex_name")
	for rows.Next() {
		rows.Scan(&schema,&table,&index)
		fmt.Printf("%s.%s\t:\t%s\n",schema,table,index)
	}
}

func unCommitTran(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()
	unCommitTran := "select threads.processlist_id,statement.sql_text,threads.processlist_command,threads.processlist_time,trx.trx_started from information_schema.innodb_trx trx,performance_schema.threads threads,performance_schema.events_statements_current statement where trx.trx_state='RUNNING' and threads.processlist_command='Sleep' and threads.processlist_time >= 2 and threads.processlist_id=trx.trx_mysql_thread_id and threads.thread_id=statement.thread_id;"
	rows, err := db.Query(unCommitTran)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var id,time int = 0,0
	var sql,cmd,trx_started = "","",""
	fmt.Println("----------UnCommit transaction information----------")
	fmt.Println("id\t:\tsql\t:\tcommand\t:\ttime\t:\ttrx_started")
	for rows.Next() {
		rows.Scan(&id,&sql,&cmd,&time,&trx_started)
		fmt.Printf("%s\t:\t%s\t:\t%s\t:\t%s\t:\t:\t%s\n",id,sql,cmd,time,trx_started)
	}
}

func innodbLockWait(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()
	lockWait := "select `r`.`trx_wait_started` as `wait_started`,timestampdiff(second,`r`.`trx_wait_started`,now()) as `wait_age_secs`,`rl`.`lock_table` as `locked_table`," +
		"`rl`.`lock_index` as `locked_index`,`r`.`trx_id` as `waiting_trx_id`,`r`.`trx_started` as `waiting_trx_started`,timediff(now(),`r`.`trx_started`) as `waiting_trx_age`," +
		"`r`.`trx_rows_locked` as `waiting_trx_rows_locked`,`r`.`trx_rows_modified` as `waiting_trx_rows_modified`,`r`.`trx_mysql_thread_id` as `waiting_pid`,`r`.`trx_query` as `waiting_query`," +
		"`rl`.`lock_mode` as `waiting_lock_mode`,`b`.`trx_id` as `blocking_trx_id`,`b`.`trx_mysql_thread_id` as `blocking_pid`,`b`.`trx_rows_locked` as `blocking_trx_rows_locked`,`b`.`trx_started` as `blocking_trx_started`," +
		"`b`.`trx_rows_modified` as `blocking_trx_rows_modified` from ((((`information_schema`.`innodb_lock_waits` `w` join " +
		"`information_schema`.`innodb_trx` `b` on((`b`.`trx_id` = `w`.`blocking_trx_id`))) join `information_schema`.`innodb_trx` `r` on((`r`.`trx_id` = `w`.`requesting_trx_id`))) join " +
		"`information_schema`.`innodb_locks` `bl` on((`bl`.`lock_id` = `w`.`blocking_lock_id`))) join `information_schema`.`innodb_locks` `rl` on((`rl`.`lock_id` = `w`.`requested_lock_id`))) order by `r`.`trx_wait_started`"
	rows, err := db.Query(lockWait)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	//var wait_started,wait_age_secs,locked_table,locked_index,waiting_trx_id,waiting_trx_started,waiting_trx_age,waiting_trx_rows_locked,waiting_trx_rows_modified
	var id,time int = 0,0
	var sql,cmd,trx_started = "","",""
	fmt.Println("----------innodb lock wait information----------")
	fmt.Println("id\t:\tsql\t:\tcommand\t:\ttime\t:\ttrx_started")
	for rows.Next() {
		rows.Scan(&id,&sql,&cmd,&time,&trx_started)
		fmt.Printf("%s\t:\t%s\t:\t%s\t:\t%s\t:\t:\t%s\n",id,sql,cmd,time,trx_started)
	}
}

func activeThread(dbUrl string)  {
	db, err := sql.Open("mysql",dbUrl)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()

}

func schemaCheck(dbur string)  {
	db, err := sql.Open("mysql",dbur)
	if err != nil {
		fmt.Printf("conn failed: %v \n",err)
		return
	}
	defer db.Close()

	engine := "select engine,count(*) num from information_Schema.tables where table_schema not in('mysql','information_schema','performance_schema','sys') and engine is not null group by engine;"
	rows, err := db.Query(engine)
	if err != nil {
		fmt.Printf("show variables exec failed , %v\n",err)
		return
	}
	var variable string = ""
	var val int = 0
	fmt.Println("------------------------------mysql table engine information------------------------------")
	fmt.Printf("%40s%40s\n","engine","num")
	for rows.Next() {
		rows.Scan(&variable,&val)
		fmt.Printf("%40s%40d\n",variable,val)
	}

	noInnodb := "select table_schema,table_name,engine from information_schema.tables where engine not in('innodb') and table_type='base table' and table_schema  not in('mysql','information_schema','performance_schema','sys')"
	rows, err = db.Query(noInnodb)
	if err != nil {
		fmt.Printf("show variables exec failed , %v\n",err)
		return
	}
	var ts,tn,en string = "","",""
	fmt.Println("------------------------------mysql not innodb engine table information-------------------")
	fmt.Printf("%40s%40s%40s\n","table_schema","table_name","engine")
	for rows.Next() {
		rows.Scan(&ts,&tn,&en)
		fmt.Printf("%40s%40s%40s\n",ts,tn,en)
	}

	character_sql := "select table_schema,table_name,COLUMN_NAME,DATA_TYPE,CHARACTER_SET_NAME,COLLATION_NAME from information_Schema.columns " +
		"where CHARACTER_SET_NAME not in('utf8','utf8mb4') and COLLATION_NAME not in('utf8mb4_bin','utf8_bin','utf8_general_ci','utf8mb4_general_ci') " +
		"and table_schema not in('mysql','information_schema','performance_schema','sys')"
	rows, err = db.Query(character_sql)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var schema,table,column,data_type,charact,collation string = "","","","","",""
	fmt.Println("----------Non-utf8[db.tablename](column_Name type character collation) information----------")
	for rows.Next() {
		rows.Scan(&schema,&table,&column,&data_type,&charact,&collation)
		fmt.Printf("%s.%s(%s %s %s %s)\n",schema,table,column,data_type,charact,collation)
	}

	dbSize := "select substring_index(name,'/',1) db,truncate(sum(FILE_SIZE)/1024/1024,0) db_size  from information_schema.INNODB_SYS_TABLESPACES " +
		"where name not like 'mysql%' and name not like 'information_schema%' and name not like 'performance_schema%' and " +
		"name not like 'sys%' and FILE_SIZE > 1024 * 1024 * 1024 group by db order by db_size desc;"
	rows, err = db.Query(dbSize)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var dbname string
	var size int
	fmt.Println("--------------------db disk size > 1G information--------------------")
	fmt.Printf("%40s%40s\n","database","size(MB)")
	for rows.Next() {
		rows.Scan(&dbname, &size)
		fmt.Printf("%40s%40d\n",dbname,size)
	}

	tableSize1 := "select name,truncate(file_size/1024/1024,0) size from information_schema.INNODB_SYS_TABLESPACES where name not like 'mysql%' and " +
		" name not like 'information_schema%' and name not like 'performance_schema%' and name not like 'sys%' and file_size>1024*1024*1024"
	rows, err = db.Query(tableSize1)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	fmt.Println("----------table size > 1024M information----------")
	fmt.Printf("%40s%40s\n","dbname/table","size(MB)")
	for rows.Next() {
		rows.Scan(&table, &size)
		fmt.Printf("%40s%40d\n",table,size)
	}

	optimizeTab := "select b.db,b.table_name,b.disk_table_size,a.table_size from (SELECT table_schema,table_name,sum(data_length+index_length)/1024/1024/1024 table_size FROM " +
		"information_schema.tables where  data_length>1*1024*1024*1024 group by table_name ) a join  (select substring_index(name,'/',1) db," +
		"substring_index(substring_index(name,'/',-1),'#',1) table_name,sum(file_size)/1024/1024/1024 disk_table_size  from information_schema.INNODB_SYS_TABLESPACES " +
		"where file_size>1*1024*1024*1024 group by table_name) b on a.table_Schema=b.db and a.table_name=b.table_name and abs(a.table_size-b.disk_table_size)/table_size>0.2;"
	rows, err = db.Query(optimizeTab)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var table_size,disk_table_size int = 0,0
	fmt.Println("----------Table size difference exceeds 20% && size > 1G ----------")
	for rows.Next() {
		rows.Scan(&schema,&table,&table_size,&disk_table_size)
		fmt.Printf("%s.%s:(schema_size) %dG , disk file size: %d G \n",schema,table,table_size,disk_table_size)
	}

	noPrimaryKey := "select t1.table_schema,t1.table_name from information_schema.tables t1 left join information_schema.table_constraints t2 on t1.table_schema = t2.table_schema " +
		"and t1.table_name = t2.table_name and t2.constraint_name ='primary' where t2.table_name is null and t1.table_schema not in ('mysql','information_schema','performance_schema','sys') " +
		"and table_type='base table' order by t1.table_schema,t1.table_name;"
	rows, err = db.Query(noPrimaryKey)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	fmt.Println("----------No primary key table(db.table) information ----------")
	//fmt.Println("database.table_name")
	for rows.Next() {
		rows.Scan(&schema,&table)
		fmt.Printf("%s.%s\n",schema,table)
	}

	redundantIndex := "select table_schema,table_name,redundant_index_name,redundant_index_non_unique,dominant_index_name,sql_drop_index from sys.schema_redundant_indexes"
	rows, err = db.Query(redundantIndex)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var index_name,unique,dominant,sql string = "","","",""
	fmt.Println("----------Redundant index table information ----------")
	fmt.Println("table_schema | table_name | redundant_index_name | redundant_index_non_unique | dominant_index_name | sql_drop_index")
	for rows.Next() {
		rows.Scan(&schema,&table,&index_name,&unique,&dominant,&sql)
		fmt.Printf("%s | %s | %s | %s | %s | %s \n",schema,table,index_name,unique,dominant,sql)
	}

	//unUseIndex56 := "select `performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_SCHEMA` AS `object_schema`,`performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_NAME` AS `object_name`," +
	//	"`performance_schema`.`table_io_waits_summary_by_index_usage`.`INDEX_NAME` AS `index_name` from `performance_schema`.`table_io_waits_summary_by_index_usage` " +
	//	"where ((`performance_schema`.`table_io_waits_summary_by_index_usage`.`INDEX_NAME` is not null) and (`performance_schema`.`table_io_waits_summary_by_index_usage`.`COUNT_STAR` = 0) " +
	//	"and (`performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_SCHEMA` not in("mysql","information_schema","sys","performance_schema")) and (`performance_schema`.`table_io_waits_summary_by_index_usage`.`INDEX_NAME` <> "PRIMARY")) " +
	//	"order by `performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_SCHEMA`,`performance_schema`.`table_io_waits_summary_by_index_usage`.`OBJECT_NAME`"
	unUseIndex57 := "select object_schema,object_name,index_name from sys.schema_unused_indexes"
	rows, err = db.Query(unUseIndex57)
	if err != nil {
		fmt.Printf("exec failed , %v\n",err)
		return
	}
	var index string = ""
	fmt.Println("----------Unused index table information----------")
	fmt.Println("database.table_name\t:\tindex_name")
	for rows.Next() {
		rows.Scan(&schema,&table,&index)
		fmt.Printf("%s.%s\t:\t%s\n",schema,table,index)
	}
}


func init(){
	flag.StringVar(&m.host,"h","127.0.0.1","Connect to mysqld `host`.")
	flag.IntVar(&m.port,"P",3306,"Connect to mysqld `port`")
	flag.StringVar(&m.user,"u","root","Connect to mysqld `user`")
	flag.StringVar(&m.passwd,"p","","Connect to mysqld `passwd`")
	flag.Usage = usage
}

func usage()  {
	fmt.Fprintf(os.Stderr,`mytunner 0.1 version
Usage: mytuner [-h host] [-P port] [-u user] [-p passwd]
Options:
`)
	flag.PrintDefaults()
}

func main()  {
	flag.Parse()
	dbur := fmt.Sprintf("%s:%s@(%s:%d)/mysql?charset=utf8",m.user,m.passwd,m.host,m.port)
	fmt.Println(dbur)
	dbStatusCheck(dbur)
	schemaCheck(dbur)
}


