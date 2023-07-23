package json

import (
	"encoding/json"
	"fmt"
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

func TestMarshal(t *testing.T) {
	var user = User{
		Username: "test",
	}
	v, _ := json.Marshal(user)
	fmt.Println(string(v))
	v, _ = Marshal(user)
	fmt.Println(string(v))
}
