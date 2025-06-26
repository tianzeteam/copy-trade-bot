package web

import (
	"copytrade/logger"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// 用户结构体
type User struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // 存储的是密码哈希
	Role     string `json:"role"`               // 用户角色：admin, user
}

// 用户存储
var users = map[string]User{}

// JWT密钥
var jwtSecret []byte

// 初始化认证系统
func InitAuth() {
	// 生成或加载JWT密钥
	jwtSecretFile := "jwt_secret.key"
	if _, err := os.Stat(jwtSecretFile); os.IsNotExist(err) {
		// 文件不存在，生成新密钥
		secret := make([]byte, 32)
		_, err := rand.Read(secret)
		if err != nil {
			logger.Fatalf("生成JWT密钥失败: %v", err)
		}
		jwtSecret = secret

		// 保存密钥到文件
		encoded := base64.StdEncoding.EncodeToString(secret)
		if err := os.WriteFile(jwtSecretFile, []byte(encoded), 0600); err != nil {
			logger.Fatalf("保存JWT密钥失败: %v", err)
		}
	} else {
		// 从文件加载密钥
		encoded, err := os.ReadFile(jwtSecretFile)
		if err != nil {
			logger.Fatalf("读取JWT密钥失败: %v", err)
		}

		secret, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			logger.Fatalf("解码JWT密钥失败: %v", err)
		}
		jwtSecret = secret
	}

	// 加载用户数据
	loadUsers()

	// 如果没有用户，创建默认管理员账户
	if len(users) == 0 {
		// 默认密码：admin123
		passwordHash, _ := bcrypt.GenerateFromPassword([]byte("admin123#000*xss"), bcrypt.DefaultCost)
		users["admin"] = User{
			Username: "admin10111",
			Password: string(passwordHash),
			Role:     "admin",
		}
		saveUsers()
		logger.Info("已创建默认管理员账户，用户名: admin10111，密码: admin123#000*xss")
	}
}

// 加载用户数据
func loadUsers() {
	usersFile := "users.json"
	if _, err := os.Stat(usersFile); os.IsNotExist(err) {
		// 文件不存在，使用空用户列表
		return
	}

	data, err := os.ReadFile(usersFile)
	if err != nil {
		logger.Errorf("读取用户数据失败: %v", err)
		return
	}

	if err := json.Unmarshal(data, &users); err != nil {
		logger.Errorf("解析用户数据失败: %v", err)
	}
}

// 保存用户数据
func saveUsers() {
	usersFile := "users.json"
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		logger.Errorf("序列化用户数据失败: %v", err)
		return
	}

	if err := os.WriteFile(usersFile, data, 0600); err != nil {
		logger.Errorf("保存用户数据失败: %v", err)
	}
}

// 登录请求结构体
type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// 登录响应结构体
type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// JWT声明结构体
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// 处理登录请求
func handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf("登录请求解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 记录登录尝试
	logger.Infof("登录尝试: 用户名='%s'", req.Username)

	// 验证用户名和密码
	user, exists := users[req.Username]
	if !exists {
		logger.Warnf("登录失败: 用户名'%s'不存在", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logger.Warnf("登录失败: 用户'%s'密码错误", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	logger.Infof("登录成功: 用户'%s'", req.Username)

	// 生成JWT令牌
	expirationTime := time.Now().Add(24 * time.Hour)
	if req.RememberMe {
		expirationTime = time.Now().Add(7 * 24 * time.Hour) // 记住我：7天
	}

	claims := &Claims{
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	// 返回令牌
	c.JSON(http.StatusOK, LoginResponse{
		Token:    tokenString,
		Username: user.Username,
		Role:     user.Role,
	})
}

// 验证令牌
func handleVerifyToken(c *gin.Context) {
	// 从请求头获取令牌
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证令牌"})
		return
	}

	// 提取令牌
	tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

	// 解析令牌
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌"})
		return
	}

	// 令牌有效
	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"username": claims.Username,
		"role":     claims.Role,
	})
}

// 修改密码请求结构体
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// 处理修改密码请求
func handleChangePassword(c *gin.Context) {
	// 获取当前用户
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 验证当前密码
	user, exists := users[username]
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户不存在"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
		return
	}

	// 更新密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码哈希失败"})
		return
	}

	user.Password = string(passwordHash)
	users[username] = user
	saveUsers()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "密码已更新"})
}

// 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取令牌
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证令牌"})
			c.Abort()
			return
		}

		// 提取令牌
		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		// 解析令牌
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("解析令牌失败: %v", err)})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌"})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// 管理员权限中间件
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// 注册认证路由
func RegisterAuthRoutes(router *gin.Engine) {
	// 公开路由
	router.POST("/api/auth/login", handleLogin)
	router.POST("/api/auth/verify", handleVerifyToken)

	// 需要认证的路由
	auth := router.Group("/api/auth")
	auth.Use(AuthMiddleware())
	{
		auth.POST("/change-password", handleChangePassword)
	}
}
