package maps

import (
	"rulego/test/assert"
	"testing"
)

type User struct {
	Username string
	Age      int
	Address  Address
}
type Address struct {
	Detail string
}

func TestMap2Struct(t *testing.T) {
	m := make(map[string]interface{})
	m["userName"] = "lala"
	m["Age"] = float64(5)
	m["Address"] = Address{"test"}
	var user User
	_ = Map2Struct(m, &user)
	assert.Equal(t, "lala", user.Username)
	assert.Equal(t, 5, user.Age)
	assert.Equal(t, "lala", user.Username)
	assert.Equal(t, "test", user.Address.Detail)
}
