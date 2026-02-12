package Macku

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
