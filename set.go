package main

type Set map[string]float32

func NewSetFromSlice(s []string) Set {
	a := make(Set)
	for _, item := range s {
		a[item] = 0
	}
	return a
}

func (set Set) IsSubsetOf(other Set) bool {
	for elem := range set {
		_, found := other[elem]
		if !found {
			return false
		}
	}
	return true
}

func (set Set) GetKeys() (s []string) {
	for k := range set {
		s = append(s, k)
	}
	return
}
