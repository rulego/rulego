package str

import (
	"reflect"
	"rulego/test/assert"
	"testing"
)

func TestSprintfDict(t *testing.T) {
	// 创建一个字典
	dict := map[string]string{
		"name": "Alice",
		"age":  "18",
	}
	// 使用SprintfDict来格式化字符串
	s := SprintfDict("Hello, ${name}. You are ${age} years old.", dict)
	assert.Equal(t, "Hello, Alice. You are 18 years old.", s)
}

func TestToString(t *testing.T) {
	var x interface{}
	x = 123 // 赋值为整数
	assert.Equal(t, "123", ToString(x))
	x = "this is test"
	assert.Equal(t, "this is test", ToString(x))
	x = []byte("this is test")
	assert.Equal(t, "this is test", ToString(x))

	x = User{Username: "lala", Age: 25}
	assert.Equal(t, "{\"Username\":\"lala\",\"Age\":25,\"Address\":{\"Detail\":\"\"}}", ToString(x))

	x = map[string]string{
		"name": "lala",
	}
	assert.Equal(t, "{\"name\":\"lala\"}", ToString(x))

}
func TestToStringMapString(t *testing.T) {
	var x interface{}
	x = map[string]interface{}{
		"name": "lala",
		"age":  5,
		"user": User{},
	}
	strMap := ToStringMapString(x)
	ageV := strMap["age"]
	ageType := reflect.TypeOf(&ageV).Elem()
	assert.Equal(t, reflect.String, ageType.Kind())

	strMap2 := ToStringMapString(strMap)
	ageV = strMap2["age"]
	ageType = reflect.TypeOf(&ageV).Elem()
	assert.Equal(t, reflect.String, ageType.Kind())
}

type User struct {
	Username string
	Age      int
	Address  Address
}
type Address struct {
	Detail string
}
