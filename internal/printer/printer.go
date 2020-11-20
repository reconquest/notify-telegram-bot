package printer

import (
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func String(data interface{}) string {
	return makeString(data, false)
}

func makeString(data interface{}, indent bool) string {
	switch typed := data.(type) {
	case map[string]interface{}:
		var keys []string
		for key := range typed {
			keys = append(keys, key)
		}

		sort.Sort(sort.StringSlice(keys))
		message := ""
		for _, key := range keys {
			value := typed[key]
			if message != "" {
				message += "\n"
			}

			if indent {
				message += "  "
			}

			message += key
			_, isInterface := value.([]interface{})
			_, isSlice := value.([]map[string]interface{})
			_, isPrimitive := value.(primitive.A)

			if isSlice || isPrimitive || isInterface {
				message += ":" + "\n"
			} else {
				message += ": "
			}

			message += makeString(value, false)
		}

		return message
	case []map[string]interface{}:
		message := ""
		for _, value := range typed {
			if message != "" {
				message += "\n\n"
			}

			message += makeString(value, true)
		}

		return message

	case primitive.A:
		message := ""
		for _, value := range typed {
			if message != "" {
				message += "\n\n"
			}

			message += makeString(value, true)
		}

		return message

	case string:
		return typed

	case []interface{}:
		message := ""
		for _, value := range typed {
			if message != "" {
				message += "\n\n"
			}

			message += makeString(value, true)
		}

		return message
	}

	return fmt.Sprint(data)

}
