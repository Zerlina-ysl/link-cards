package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// 全局日志实例
var Log *logrus.Logger

func init() {
	// 创建 logrus 实例
	Log = logrus.New()
	
	// 设置日志格式
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		DisableColors:   true,
	})

	// 设置日志级别
	Log.SetLevel(logrus.InfoLevel)

	// 默认输出到控制台
	Log.SetOutput(os.Stdout)
}

// SetupLogger 初始化日志系统
func SetupLogger() error {
	// 创建 logs 目录
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 创建应用日志文件（按日期）
	appFileName := filepath.Join(logDir, fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02")))
	appFile, err := os.OpenFile(appFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// 创建访问日志文件
	accessFileName := filepath.Join(logDir, fmt.Sprintf("access_%s.log", time.Now().Format("2006-01-02")))
	accessFile, err := os.OpenFile(accessFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// 设置主日志同时输出到控制台和文件
	Log.SetOutput(io.MultiWriter(os.Stdout, appFile))

	// 创建独立的 access log
	accessLog := logrus.New()
	accessLog.SetOutput(accessFile)
	accessLog.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		DisableColors:   true,
	})
	accessLog.SetLevel(logrus.InfoLevel)
	
	// 保存 access log 到全局变量供中间件使用
	accessLogger = accessLog

	Log.Infof("日志系统初始化完成")
	Log.Infof("日志目录: %s", logDir)
	Log.Infof("应用日志: %s", appFileName)
	Log.Infof("访问日志: %s", accessFileName)

	return nil
}

// 访问日志实例
var accessLogger *logrus.Logger

// LogRequest 请求日志中间件
func LogRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()

		logMsg := fmt.Sprintf("%s %s %d %v %s", method, path, status, latency, clientIP)

		// 记录到 access log
		if accessLogger != nil {
			accessLogger.Info(logMsg)
		}
		// 同时记录到主日志
		Log.Info(logMsg)
	}
}

// LogInfo 记录信息日志
func LogInfo(format string, v ...interface{}) {
	Log.Infof(format, v...)
}

// SetLogLevel 设置日志级别
func SetLogLevel(level string) {
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
}

// LogError 记录错误日志
func LogError(format string, v ...interface{}) {
	Log.Errorf(format, v...)
}

// LogParseResult 记录解析结果
func LogParseResult(url string, count int, err error) {
	if err != nil {
		Log.Errorf("解析失败: url=%s error=%v", url, err)
	} else {
		Log.Infof("解析成功: url=%s count=%d", url, count)
	}
}

// LogServerStart 记录服务启动
func LogServerStart(addr string) {
	Log.Infof("========================================")
	Log.Infof("服务启动于 http://%s", addr)
	Log.Infof("========================================")
}
