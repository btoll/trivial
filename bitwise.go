package trivial

import (
	"fmt"
	"math"
	"strconv"
)

//https://play.golang.com/p/s4OQPYxfGj3

func getAnswer(total uint16, nums []uint16) []uint16 {
	k := len(nums) - 1
	res := []uint16{nums[k]}
	sum := nums[k]
	k -= 1
	for k >= 0 && sum != total {
		if (sum + nums[k]) <= total {
			sum += nums[k]
			res = append(res, nums[k])
		}
		k -= 1
	}
	return res
}

func getBase2Components(bitmap uint16) []uint16 {
	if isBase2(bitmap) {
		return []uint16{bitmap}
	}
	var n uint16 = 1
	s := []uint16{}
	for n < bitmap {
		s = append(s, n)
		n <<= 1
	}
	return s
}

// This is awful, I'm sorry.
func getItemFromLog(choices []string, num uint16) []string {
	n := removeBit(num)
	possibleAnswers := getBase2Components(n)
	correctIndices := getAnswer(n, possibleAnswers)
	t := []string{}
	for _, v := range correctIndices {
		log := math.Log2(float64(v))
		t = append(t, choices[int(log)])
	}
	return t
}

func isBase2(n uint16) bool {
	return (n & (n - 1)) == 0
}

func makeBitmap(nums []string) uint16 {
	var total uint16
	for _, d := range nums {
		n, err := strconv.ParseUint(d, 10, 16)
		if err != nil {
			fmt.Sprintln("%s cannot be converted to an integer, ignoring\n", n)
			continue
		}
		// The answers are entered in as one-based in the CSV.
		// TODO: Should this be done here or in the caller?
		total += 1 << (n - 1)
	}
	return total
}

func removeBit(n uint16) uint16 {
	if (n & (1 << 15)) == 1<<15 {
		return n - 1<<15
	}
	return n
}
