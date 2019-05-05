package main

import (
	"errors"
	"fmt"
)

const LRU = "LRU"
const LFU = "LFU"

//TODO figure out parameters
type Func func(key string) ([]byte, error)

//TODO Figure out parameters
type Policy struct {
	f Func
	expirationTime uint8
}

func NewPolicy(replacementPolicy string, expirationTime uint8) (policy *Policy, err error) {
	switch replacementPolicy {
	case LRU:
		policy = &Policy{f:nil, expirationTime: expirationTime}
	case LFU:
		policy = &Policy{f:nil, expirationTime: expirationTime}
	default:
		err = errors.New(fmt.Sprintf("Invalid cache replacement policy [%s]", replacementPolicy))
	}
	return
}