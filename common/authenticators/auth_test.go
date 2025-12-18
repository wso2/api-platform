package authenticators_test

import (
"encoding/base64"
"net/http"
"net/http/httptest"
"testing"

"github.com/gin-gonic/gin"
"github.com/wso2/api-platform/common/authenticators"
"github.com/wso2/api-platform/common/models"
"golang.org/x/crypto/bcrypt"
)

func init() {
gin.SetMode(gin.TestMode)
}

func TestBasicAuthSuccess(t *testing.T) {
r := gin.New()

basicAuth := &models.BasicAuth{
Enabled: true,
Users: []models.User{
{
UserID:         "testuser",
Password:       "testpass",
PasswordHashed: false,
Roles:          []string{"user"},
},
},
}

r.Use(authenticators.BasicAuthMiddleware(basicAuth, nil))

r.GET("/test", func(c *gin.Context) {
userID, exists := authenticators.GetUserIDFromContext(c)
if !exists {
t.Error("User ID not found in context")
}
if userID != "testuser" {
t.Errorf("Expected user_id 'testuser', got '%s'", userID)
}
c.JSON(200, gin.H{"status": "ok"})
})

w := httptest.NewRecorder()
req, _ := http.NewRequest("GET", "/test", nil)
credentials := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
req.Header.Set("Authorization", "Basic "+credentials)

r.ServeHTTP(w, req)

if w.Code != 200 {
t.Errorf("Expected status 200, got %d", w.Code)
}
}

func TestBasicAuthFailure(t *testing.T) {
r := gin.New()

basicAuth := &models.BasicAuth{
Enabled: true,
Users: []models.User{
{
UserID:   "testuser",
Password: "testpass",
},
},
}

r.Use(authenticators.BasicAuthMiddleware(basicAuth, nil))

r.GET("/test", func(c *gin.Context) {
c.JSON(200, gin.H{"status": "ok"})
})

w := httptest.NewRecorder()
req, _ := http.NewRequest("GET", "/test", nil)
credentials := base64.StdEncoding.EncodeToString([]byte("testuser:wrongpass"))
req.Header.Set("Authorization", "Basic "+credentials)

r.ServeHTTP(w, req)

if w.Code != 401 {
t.Errorf("Expected status 401, got %d", w.Code)
}
}

func TestBasicAuthBcrypt(t *testing.T) {
r := gin.New()

hashedPassword, err := bcrypt.GenerateFromPassword([]byte("secure-password"), bcrypt.DefaultCost)
if err != nil {
t.Fatal(err)
}

basicAuth := &models.BasicAuth{
Enabled: true,
Users: []models.User{
{
UserID:         "user1",
Password:       string(hashedPassword),
PasswordHashed: true,
Roles:          []string{"user"},
},
},
}

r.Use(authenticators.BasicAuthMiddleware(basicAuth, nil))

r.GET("/data", func(c *gin.Context) {
username, _ := authenticators.GetUsernameFromContext(c)
c.JSON(200, gin.H{"username": username})
})

w := httptest.NewRecorder()
req, _ := http.NewRequest("GET", "/data", nil)
credentials := base64.StdEncoding.EncodeToString([]byte("user1:secure-password"))
req.Header.Set("Authorization", "Basic "+credentials)

r.ServeHTTP(w, req)

if w.Code != 200 {
t.Errorf("Expected status 200, got %d", w.Code)
}
}

func TestSkipPaths(t *testing.T) {
r := gin.New()

basicAuth := &models.BasicAuth{
Enabled: true,
Users:   []models.User{{UserID: "user", Password: "pass"}},
}

r.Use(authenticators.BasicAuthMiddleware(basicAuth, []string{"/health", "/metrics"}))

r.GET("/health", func(c *gin.Context) {
c.JSON(200, gin.H{"status": "ok"})
})

r.GET("/protected", func(c *gin.Context) {
c.JSON(200, gin.H{"data": "secret"})
})

w1 := httptest.NewRecorder()
req1, _ := http.NewRequest("GET", "/health", nil)
r.ServeHTTP(w1, req1)

if w1.Code != 200 {
t.Errorf("Expected status 200 for /health, got %d", w1.Code)
}

w2 := httptest.NewRecorder()
req2, _ := http.NewRequest("GET", "/protected", nil)
r.ServeHTTP(w2, req2)

if w2.Code != 401 {
t.Errorf("Expected status 401 for /protected, got %d", w2.Code)
}
}
