package vlog

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const defaultDatabaseConnMaxIdleTime = time.Minute * 2

type databaseWriter struct {
	io.WriteCloser
	lock                    sync.Mutex
	dbType                  string  //数据库类型
	connUrl                 string  //数据库连接url
	tableName               string  //写入数据表名
	conn                    *dbConn //封装的数据库连接（Connection）
	isNeedAutoFreeDBConn    bool    //自动释放数据库连接开关
	lastAutoFreeDBConnTimer *time.Timer
}

type dbConn struct {
	conn           *sql.DB
	lastAccessTime time.Time
}

func (conn *dbConn) isExpired() bool {
	nowTime := time.Now()
	if nowTime.After(conn.lastAccessTime.Add(defaultDatabaseConnMaxIdleTime)) {
		return true
	}
	return false
}

func (conn *dbConn) String() string {
	if conn != nil {
		return fmt.Sprint("dbConn: lastAccessTime=", conn.lastAccessTime)
	}
	return ""
}

func newDababaseWriter(dbType, connUrl, tableName string) (dbWriter *databaseWriter, err error) {
	if dbType != "mysql" {
		return nil, errors.New("databaseWriter only supports MySQL")
	}
	dbWriter = new(databaseWriter)
	dbWriter.dbType = dbType
	dbWriter.connUrl = connUrl
	dbWriter.tableName = tableName
	//必须为true
	dbWriter.isNeedAutoFreeDBConn = true
	return dbWriter, nil
}

func (dbWriter *databaseWriter) newDBConn() (conn *dbConn, err error) {
	conn = new(dbConn)
	conn.conn, err = sql.Open(dbWriter.dbType, dbWriter.connUrl)
	if err != nil {
		return nil, err
	}
	conn.lastAccessTime = time.Now()
	if dbWriter.isNeedAutoFreeDBConn {
		if dbWriter.lastAutoFreeDBConnTimer != nil {
			dbWriter.lastAutoFreeDBConnTimer.Stop()
			dbWriter.lastAutoFreeDBConnTimer = nil
		}
		dbWriter.lastAutoFreeDBConnTimer = time.AfterFunc(defaultDatabaseConnMaxIdleTime,
			dbWriter.autoFreeExpiredConn)
	}
	return conn, nil
}

func (dbWriter *databaseWriter) autoFreeExpiredConn() {
	if dbWriter.conn.isExpired() {
		//已过期，清理
		dbWriter.Close()
	} else {
		//未过期，等会儿再检查
		dbWriter.lastAutoFreeDBConnTimer = time.AfterFunc(defaultDatabaseConnMaxIdleTime,
			dbWriter.autoFreeExpiredConn)
	}
}

func (dbWriter *databaseWriter) Write(bytes []byte) (n int, err error) {
	
	if dbWriter.conn == nil {
		dbWriter.conn, err = dbWriter.newDBConn()
		if err != nil {
			return 0, err
		}
	}
	dbWriter.conn.lastAccessTime = time.Now()
	//这里不负责关闭数据库连接
	conn := dbWriter.conn.conn
	sqlStr := "insert into " + dbWriter.tableName + "(content) values(?)"
	res, err := conn.Exec(sqlStr, string(bytes))
	if err != nil {
		return 0, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if ra <= 0 {
		return 0, errors.New("no data inserted")
	}
	dbWriter.conn.lastAccessTime = time.Now()
	return len(bytes), nil
}

func (dbWriter *databaseWriter) Close() error {
	dbWriter.lock.Lock()
	defer dbWriter.lock.Unlock()
	if dbWriter.conn != nil {
		err := dbWriter.conn.conn.Close()
		//不论关闭成功与否，此dbConn是不能用了
		dbWriter.conn = nil
		return err
	}
	return nil
}

func (dbWriter *databaseWriter) String() string {
	return "databaseWriter: dbType=" + dbWriter.dbType +
		", connUrl=" + dbWriter.connUrl +
		", tableName=" + dbWriter.tableName +
		", conn=[" + fmt.Sprint(dbWriter.conn, "]")
}
