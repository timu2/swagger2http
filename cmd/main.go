package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"swagger2http"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: swagger json file")
		return
	}

	f := os.Args[1]

	//f := "petstore.json"
	s := &swagger2http.Swagger{}
	err := s.Load(f)
	if err != nil {
		fmt.Println(err)
		return
	}

	out, err := s.Dump()
	if err != nil {
		fmt.Println(err)
		return
	}

	if out.Len() == 0 {
		fmt.Println("empty result")
		return
	}

	_ = ioutil.WriteFile(f+".http", out.Bytes(), 0644)
}
