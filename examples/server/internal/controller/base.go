package controller

import (
	"examples/server/config"
	"examples/server/internal/constants"
	"examples/server/internal/model"
	"examples/server/internal/service"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/json"
)

var Base = &base{}

type base struct {
}

type RuleGoClaim struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.StandardClaims
}

// userNotFound 用户不存在
func userNotFound(username string, exchange *endpointApi.Exchange) bool {
	exchange.Out.SetStatusCode(http.StatusBadRequest)
	exchange.Out.SetBody([]byte("no found username for:" + username))
	return false
}

// unauthorized 用户未授权
func unauthorized(username string, exchange *endpointApi.Exchange) bool {
	exchange.Out.SetStatusCode(http.StatusUnauthorized)
	exchange.Out.SetBody([]byte("unauthorized for:" + username))
	return false
}

// GetRuleGoFunc 动态获取指定用户规则链池
func GetRuleGoFunc(exchange *endpointApi.Exchange) types.RuleEnginePool {
	msg := exchange.In.GetMsg()
	username := msg.Metadata.GetValue(constants.KeyUsername)
	if s, ok := service.UserRuleEngineServiceImpl.Get(username); !ok {
		exchange.In.SetError(fmt.Errorf("not found username=%s", username))
		return engine.DefaultPool
	} else {
		return s.Pool
	}
}

var AuthProcess = func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
	if !config.Get().RequireAuth {
		//允许匿名访问
		msg := exchange.In.GetMsg()
		msg.Metadata.PutValue(constants.KeyUsername, config.C.DefaultUsername)
		return true
	}
	claim, err := parseToken(exchange.In.Headers().Get(constants.KeyAuthorization))
	if err != nil {
		exchange.Out.SetStatusCode(http.StatusUnauthorized)
		exchange.Out.SetBody([]byte(err.Error()))
		return false
	}
	msg := exchange.In.GetMsg()
	msg.Metadata.PutValue(constants.KeyUsername, claim.Username)
	return true
}

func parseToken(token string) (*RuleGoClaim, error) {
	if len(token) == 0 {
		return nil, fmt.Errorf("token is empty")
	}
	token = token[len(constants.KeyBearer):]
	claims := &RuleGoClaim{}
	tk, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.C.JwtSecretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := tk.Claims.(*RuleGoClaim); ok && tk.Valid {
		return claims, nil
	} else {
		return nil, fmt.Errorf("token is invalid")
	}
}

func (c *base) Login(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		var user model.User
		if err := json.Unmarshal([]byte(msg.Data), &user); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			if b := validatePassword(user); b {
				claim := RuleGoClaim{
					Username: user.Username,
					StandardClaims: jwt.StandardClaims{
						ExpiresAt: time.Now().Add(time.Duration(config.C.JwtExpireTime) * time.Millisecond).Unix(), // 设置 Token 过期时间
						Issuer:    config.C.JwtIssuer,                                                              // 设置 Token 的签发者
					},
				}
				token, err := createToken(claim)
				if err != nil {
					exchange.Out.SetStatusCode(http.StatusInternalServerError)
					exchange.Out.SetBody([]byte(err.Error()))
				}
				result, err := json.Marshal(map[string]interface{}{
					"token":     *token,
					"expiresAt": claim.ExpiresAt,
				})
				if err != nil {
					exchange.Out.SetStatusCode(http.StatusInternalServerError)
					exchange.Out.SetBody([]byte(err.Error()))
				} else {
					exchange.Out.SetBody(result)
				}
				return true

			} else {
				return unauthorized(user.Username, exchange)
			}
		}
		return true
	}).End()
}

func createToken(claim jwt.Claims) (*string, error) {
	// 创建 JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenString, err := token.SignedString([]byte(config.Get().JwtSecretKey))
	if err != nil {
		fmt.Printf("Error generating token: %v\n", err)
		return nil, err
	}
	return &tokenString, nil
}

func validatePassword(user model.User) bool {
	users := config.Get().Users
	if users != nil && config.Get().Users[user.Username] == user.Password {
		return true
	}
	return false
}
