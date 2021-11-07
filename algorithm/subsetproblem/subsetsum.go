/*
	This subsetsum.go: 子集和问题
	// Input: 	给出一个数组array和一给定数字sum
	// Out: 	数组中累加和为sum的元素
*/
package subsetproblem

import (
	"bytes"
	"fmt"
)

func printArr(arr [][]bool) {
	str := bytes.Buffer{}
	str.WriteString("\n")
	for i := range arr {
		str.WriteString("{ ")
		for j := range arr[i] {
			str.WriteString(fmt.Sprintf("[%d,%d]%v\t", i, j, arr[i][j]))
		}
		str.WriteString("}\n")
	}
	fmt.Printf(str.String())
}

// 返回数组下标
func GetSubset(set []int, sum int) []int {
	length := len(set)
	subSet := make([][]bool, length+1, length+1)
	for i := range subSet {
		subSet[i] = make([]bool, sum+1, sum+1)
	}

	// If sum is 0, then answer is true
	for i := 0; i <= length; i++ {
		subSet[i][0] = true
	}

	// If sum is not 0 and set is empty, then answer is false
	for i := 1; i <= sum; i++ {
		subSet[0][i] = false
	}

	// Fill the subset table in bottom up manner
	for i := 1; i <= length; i++ {
		for j := 1; j <= sum; j++ {
			if set[i-1] == sum {
				// 自己就满足
				return []int{i - 1}
			}
			if j < set[i-1] {
				subSet[i][j] = subSet[i-1][j]
			}
			if j >= set[i-1] {
				subSet[i][j] = subSet[i-1][j] || subSet[i-1][j-set[i-1]]
			}
		}
	}

	//printArr(subSet)

	if !subSet[length][sum] {
		//fmt.Printf("Do not have match list.\n")
		return nil
	}

	var result []int
	tmpTargetNum := sum
	for j := sum; j > 0; j-- {
		if j != tmpTargetNum {
			continue
		}
		for i := length; i >= 0; i-- {
			if !subSet[i][j] && i+1 <= length && subSet[i+1][j] {
				// 当前数字是目标之一
				result = append(result, i) // sum[i] means sum[(i+1 -1)]
				// 更新target
				tmpTargetNum -= set[i]
				break
			}
		}
	}

	return result
}
