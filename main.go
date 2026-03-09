package main

import (
	"embed"
	"flag"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed templates/* static/*
var templates embed.FS

func main() {
	// 解析命令行参数
	mode := flag.String("mode", "test", "Gin 运行模式: test, release")
	flag.Parse()

	// 初始化日志系统
	if err := SetupLogger(); err != nil {
		LogInfo("日志系统初始化失败: %v", err)
	}

	// 设置 gin 模式
	switch *mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
		SetLogLevel("info")
	case "test":
		gin.SetMode(gin.TestMode)
		SetLogLevel("debug")
	default:
		gin.SetMode(gin.TestMode)
		SetLogLevel("debug")
	}

	LogInfo("Gin 运行模式: %s", *mode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(LogRequest())

	// CORS 中间件 - 允许浏览器扩展访问
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 设置静态文件
	staticFS, _ := fs.Sub(templates, "static")
	r.StaticFS("/static", http.FS(staticFS))

	// 加载 HTML 模板
	r.LoadHTMLGlob("templates/*")

	// 路由
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/view/:id", func(c *gin.Context) {
		id := c.Param("id")
		data := GetCardData(id)
		if data == nil {
			c.String(http.StatusNotFound, "页面不存在")
			return
		}
		c.HTML(http.StatusOK, "card.html", data)
	})

	api := r.Group("/api")
	{
		api.POST("/parse", HandleParse)
		api.POST("/parse-from-extension", HandleParseFromExtension)
		api.GET("/list", HandleList)
	}

	// 记录启动日志
	addr := ":8080"
	LogServerStart(addr)
	
	// 启动服务
	if err := r.Run(addr); err != nil {
		LogError("服务启动失败: %v", err)
	}
}
