package str

import (
	"fmt"
	"math/rand"
	"rulego/utils/json"
	"strconv"
	"strings"
)

const varPatternLeft = "${"
const varPatternRight = "}"

// SprintfDict formats a string according to a pattern and a dictionary of variables.
//The pattern is a string that contains placeholders for the variables in the form of ${key}.
//The dict is a map from keys to values that will replace the placeholders.
//For example, SprintfDict(“Hello, ${name}!”, map[string]string{“name”: “Alice”}) returns “Hello, Alice!”.
//If the pattern contains a key that is not in the dict, it will be left unchanged.
//If the dict contains a key that is not in the pattern, it will be ignored.
func SprintfDict(pattern string, dict map[string]string) string {
	var result = pattern
	for key, value := range dict {
		result = ProcessVar(result, key, value)
	}
	return result
}

func ProcessVar(pattern, key, val string) string {
	varPattern := varPatternLeft + key + varPatternRight
	return strings.Replace(pattern, varPattern, val, -1)
}

const randomStrOptions = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const randomStrOptionsLen = len(randomStrOptions)

//RandomStr 创建指定长度的随机字符
func RandomStr(num int) string {
	var builder strings.Builder
	for i := 0; i < num; i++ {
		builder.WriteByte(randomStrOptions[rand.Intn(randomStrOptionsLen)])
	}
	return builder.String()
}

// ToString input的值转成字符串
func ToString(input interface{}) string {
	if input == nil {
		return ""
	}
	switch v := input.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		ft := input.(float64)
		return strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		ft := input.(float32)
		return strconv.FormatFloat(float64(ft), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case uint:
		return strconv.Itoa(int(v))
	case int8:
		return strconv.Itoa(int(v))
	case uint8:
		return strconv.Itoa(int(v))
	case int16:
		return strconv.Itoa(int(v))
	case uint16:
		return strconv.Itoa(int(v))
	case int32:
		return strconv.Itoa(int(v))
	case uint32:
		return strconv.Itoa(int(v))
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	case error:
		return v.Error()
	default:
		if newValue, err := json.Marshal(input); err == nil {
			return string(newValue)
		} else {
			return ""
		}
	}
}

//ToStringMapString 把interface类型 转 map[string]string类型
func ToStringMapString(input interface{}) map[string]string {
	var output = map[string]string{}

	switch v := input.(type) {
	case map[string]string:
		return v
	case map[string]interface{}:
		for k, val := range v {
			output[ToString(k)] = ToString(val)
		}
		return output
	case map[interface{}]string:
		for k, val := range v {
			output[ToString(k)] = ToString(val)
		}
		return output
	case map[interface{}]interface{}:
		for k, val := range v {
			output[ToString(k)] = ToString(val)
		}
		return output
	case string:
		_ = json.Unmarshal([]byte(v), &output)
		return output
	default:
		return output
	}
}
