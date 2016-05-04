package testing

import "fmt"

type a struct {
	value string
}

func main() {
	t := &a{}
	t.value = "test"
	fmt.Println(t.value)
}
