package scenario

import (
	"strconv"
	"strings"
)

const (
	rowsSeparator  = ","
	rangeSeparator = "-"
)

func parseNums(usersStr string) ([]int, error) {
	arr := strings.Split(usersStr, rowsSeparator)
	var users []int
	for _, s := range arr {
		if strings.Contains(s, rangeSeparator) {
			subArr := strings.SplitN(s, rangeSeparator, 2)
			first, err := strconv.Atoi(subArr[0])
			if err != nil {
				return []int{}, err
			}
			last, err := strconv.Atoi(subArr[1])
			if err != nil {
				return []int{}, err
			}
			for user := first; user <= last; user++ {
				users = append(users, user)
			}
		} else {
			user, err := strconv.Atoi(s)
			if err != nil {
				return []int{}, err
			}
			users = append(users, user)
		}
	}
	return users, nil
}
