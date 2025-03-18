package model

// Unused

import (
	"strings"
)

const CONFIG_SEPARATOR = "%"

type ConfigGrouper interface {
	Add(string)
	AddElement(ConfigElementer)
	Get(string) (ConfigGrouper, bool)
	String() string
	StringLines() []string
}

type ConfigGroup struct {
	Title string
	Depth int

	mapper map[string]ConfigGrouper
	slice  []ConfigElementer
}

func newConfigGroup(title string, depth int) *ConfigGroup {
	return &ConfigGroup{Title: title, Depth: depth, mapper: map[string]ConfigGrouper{}}
}

func (g *ConfigGroup) Add(line string) {
	if strings.Contains(line, CONFIG_SEPARATOR) {
		tmp := strings.SplitN(line, CONFIG_SEPARATOR, 2)
		g2, ok := g.Get(tmp[0])
		if !ok {
			g2 = newConfigGroup(tmp[0], g.Depth+1)
			g.AddElement(g2)
		}
		g2.Add(tmp[1])
	} else {
		e := newConfigElement(line)
		g.AddElement(e)
	}
}

func (g *ConfigGroup) AddElement(e ConfigElementer) {
	if g2, ok := e.(*ConfigGroup); ok {
		g.mapper[g2.Title] = g2
		g2.Depth = g.Depth + 1
	}
	g.slice = append(g.slice, e)
}

func (g *ConfigGroup) Get(title string) (ConfigGrouper, bool) {
	g2, ok := g.mapper[title]
	return g2, ok
}

func (g *ConfigGroup) String() string {
	return strings.Join(g.StringLines(), "\n")
}

func (g *ConfigGroup) StringLines() []string {
	buf := []string{g.Title + " {"}
	for _, e := range g.slice {
		for _, s := range e.StringLines() {
			buf = append(buf, "\t"+s)
		}
	}
	buf = append(buf, "}")
	return buf
}

type ConfigElementer interface {
	String() string
	StringLines() []string
}

type ConfigElement struct {
	Line string
}

func newConfigElement(line string) *ConfigElement {
	return &ConfigElement{Line: line}
}

func (e *ConfigElement) String() string {
	return e.Line
}

func (e *ConfigElement) StringLines() []string {
	return []string{e.String()}
}

// cisco-like configs

//type ciscoConfigGroup struct{ *ConfigGroup }
//
//func (g *ciscoConfigGroup) StringLines() []string {
//	buf := []string{g.Title}
//	for _, e := range g.slice {
//		for _, s := range e.StringLines() {
//			buf = append(buf, " "+s)
//		}
//		if g.Depth == 0 {
//			buf = append(buf, "!")
//		}
//	}
//	return buf
//}
//
//type ciscoConfigLine struct{ *ConfigElement }
//
//func newCiscoConfigRoot() *ciscoConfigGroup {
//	return &ciscoConfigGroup{ConfigGroup: newConfigGroup("", 0)}
//}
//
//func mergeConfigCisco(blocks []*configBlock) ([]string, error) {
//	root := newCiscoConfigRoot()
//	for _, block := range blocks {
//		lines := strings.Split(block.config, "\n")
//		for _, line := range lines {
//			root.Add(line)
//		}
//	}
//
//	return root.StringLines(), nil
//}
//
//func mergeConfigFRR(blocks []*configBlock) ([]string, error) {
//	return mergeConfigCisco(blocks)
//}
