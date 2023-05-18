package model

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
)

// labelOwner includes Node, Interface, Connection
type labelOwner interface {
	ClassLabels() []string
	PlaceLabels() []string
	ValueLabels() map[string]string
	MetaValueLabels() map[string]string
}

type parsedLabels struct {
	classLabels     []string
	placeLabels     []string
	valueLabels     map[string]string
	metaValueLabels map[string]string
}

func (l parsedLabels) ClassLabels() []string {
	return l.classLabels
}

func (l parsedLabels) PlaceLabels() []string {
	return l.placeLabels
}

func (l parsedLabels) ValueLabels() map[string]string {
	return l.valueLabels
}

func (l parsedLabels) MetaValueLabels() map[string]string {
	return l.metaValueLabels
}

// addressOwner includes Node, Interface
// commented out because it currently does not have abstracted usage (explicitly addressed)
// type addressOwner interface {
// 	setAware(string)
// 	IsAware(string) bool
// }

type addressedObject struct {
	awareLayers mapset.Set[string]
}

func newAddressedObject() addressedObject {
	return addressedObject{
		awareLayers: mapset.NewSet[string](),
	}
}

func (a addressedObject) setAware(layer string) {
	a.awareLayers.Add(layer)
}

func (a addressedObject) IsAware(layer string) bool {
	return a.awareLayers.Contains(layer)
}

// NameSpacer includes Node, Interface, Neighbor, Group
type NameSpacer interface {
	setNumbered(k string)
	isNumbered(k string) bool
	iterNumbered() <-chan string
	addNumber(k, v string)
	hasNumber(k string) bool
	setNumbers(map[string]string)
	setRelativeNumber(k, v string)
	hasRelativeNumber(k string) bool
	setRelativeNumbers(map[string]string)
	GetNumbers() map[string]string
	GetRelativeNumbers() map[string]string
	GetValue(string) (string, error)
}

type NameSpace struct {
	numbered        mapset.Set[string]
	numbers         map[string]string
	relativeNumbers map[string]string
}

func newNameSpace() *NameSpace {
	return &NameSpace{
		numbered:        mapset.NewSet[string](),
		numbers:         map[string]string{},
		relativeNumbers: map[string]string{},
	}
}

func (ns *NameSpace) setNumbered(k string) {
	ns.numbered.Add(k)
}

func (ns *NameSpace) isNumbered(k string) bool {
	return ns.numbered.Contains(k)
}

func (ns *NameSpace) iterNumbered() <-chan string {
	return ns.numbered.Iter()
}

func (ns *NameSpace) addNumber(k, v string) {
	ns.numbers[k] = v
}

func (ns *NameSpace) hasNumber(k string) bool {
	_, ok := ns.numbers[k]
	return ok
}

func (ns *NameSpace) setNumbers(given map[string]string) {
	if len(ns.numbers) == 0 {
		ns.numbers = given
	} else {
		for k, v := range given {
			ns.numbers[k] = v
		}
	}
}

func (ns *NameSpace) GetNumbers() map[string]string {
	return ns.numbers
}

func (ns *NameSpace) setRelativeNumber(k, v string) {
	ns.relativeNumbers[k] = v
}

func (ns *NameSpace) hasRelativeNumber(k string) bool {
	_, ok := ns.relativeNumbers[k]
	return ok
}

func (ns *NameSpace) setRelativeNumbers(given map[string]string) {
	if len(ns.relativeNumbers) == 0 {
		ns.relativeNumbers = given
	} else {
		for k, v := range given {
			ns.relativeNumbers[k] = v
		}
	}
}

func (ns *NameSpace) GetRelativeNumbers() map[string]string {
	return ns.relativeNumbers
}

func (ns *NameSpace) GetValue(key string) (string, error) {
	val, ok := ns.relativeNumbers[key]
	if ok {
		return val, nil
	} else {
		return val, fmt.Errorf("unknown key %v", key)
	}
}
