package printer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	text = "latest:\n" +
		"  company: Cisco Systems Inc.\n" +
		"  tier: 6000 Users\n\n" +
		"  company: Cisco Systems Inc.\n" +
		"  tier: 1000 Users\n\n" +
		"  company: Cisco Systems Inc.\n" +
		"  tier: 1001 Users\n\n" +
		"  company: Cisco Systems Inc.\n" +
		"  tier: 1002 Users\n\n" +
		"  company: Cisco Systems Inc.\n" +
		"  tier: 1003 Users\n" +
		"total: 1243"
)

func Test_String_ReturnStringData(t *testing.T) {
	data := map[string]interface{}{
		"total": "1243",
		"latest": []map[string]interface{}{
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "6000 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1000 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1001 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1002 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1003 Users",
			},
		},
	}

	message := String(data)

	assert.Equal(
		t,
		strings.Split(text, "\n"),
		strings.Split(message, "\n"),
		"should contain matching text",
	)
}

func Test_String_ReturnStringDataWithPrimitiveA(t *testing.T) {
	data := map[string]interface{}{
		"latest": primitive.A{
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "6000 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1000 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1001 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1002 Users",
			},
			map[string]interface{}{
				"company": "Cisco Systems Inc.",
				"tier":    "1003 Users",
			},
		},
		"total": "1243"}

	message := String(data)
	assert.Equal(
		t,
		strings.Split(text, "\n"),
		strings.Split(message, "\n"),
		"should contain matching text",
	)
}

func Test_String_ReturnStringDataByURL(t *testing.T) {
	data := []byte(`{"metrics":{"total":350,"latest":[{"addon":"","company":"Cartwright, Ondricka and Deckow","tier":"500 Users","price":494.5},{"addon":"","company":"Mante, Doyle and Spencer","tier":"1000 Users","price":1600},{"addon":"","company":"Jerde-Cummerata","tier":"25 Users","price":79},{"addon":"","company":"Sporer-Von","tier":"1000 Users","price":1599},{"addon":"","company":"Denesik, Schoen and Swaniawski","tier":"10 Users","price":10}]}}`)
	var jsonData map[string]interface{}
	err := json.Unmarshal(data, &jsonData)
	assert.NoError(t, err)

	expected := `metrics: latest:
  addon: 
  company: Cartwright, Ondricka and Deckow
  price: 494.5
  tier: 500 Users

  addon: 
  company: Mante, Doyle and Spencer
  price: 1600
  tier: 1000 Users

  addon: 
  company: Jerde-Cummerata
  price: 79
  tier: 25 Users

  addon: 
  company: Sporer-Von
  price: 1599
  tier: 1000 Users

  addon: 
  company: Denesik, Schoen and Swaniawski
  price: 10
  tier: 10 Users
total: 350`

	message := String(jsonData)

	assert.Equal(
		t,
		expected,
		message,
	)

}
