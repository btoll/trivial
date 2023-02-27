package trivial

import (
	"fmt"
	"math"
	"strconv"
)

//https://play.golang.com/p/s4OQPYxfGj3
//https://play.golang.com/p/F8bDipTDhjK

// Get the parts of the sum.
func getBase2Components(total uint16) []uint16 {
	if isBase2(total) {
		return []uint16{total}
	}
	var n uint16 = 1
	var k uint16 = 0
	// Get the largest power of 2 that "fits" in the integer.
	for n < total {
		k = n
		n <<= 1
	}
	sum := k
	// The largest power of 2 will be the first thing to append.
	res := []uint16{k}
	// Get the logarithm of the largest power of 2.  This is what
	// will be used to bitshift left and get the remaining power
	// of 2s in descending order.
	j := uint16(math.Log2(float64(k - 1)))
	for j >= 0 && sum != total {
		if sum+1<<j <= total {
			sum += 1 << j
			res = append(res, 1<<j)
		}
		j -= 1
	}
	return res
}

// This is (less) awful (than before), I'm sorry.
func getItemFromLog(choices []string, num uint16) []string {
	correctIndices := getBase2Components(removeLastBit(num))
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

func removeLastBit(n uint16) uint16 {
	if (n & (1 << 15)) == 1<<15 {
		return n - 1<<15
	}
	return n
}
