package amqp_kit

import "strings"

const (
	delimeter = "."
	matchWord = "*"
	matchRest = "#"
)

func match(key, mask string) bool {

	mparts := strings.Split(mask, delimeter)
	kparts := strings.Split(key, delimeter)

	if len(mparts) > len(kparts) {
		return false
	}

	for i, p := range mparts {
		switch p {
		case matchWord:
		case matchRest:
			break
		default:
			if p != kparts[i] {
				return false
			}
		}
	}

	return true
}
