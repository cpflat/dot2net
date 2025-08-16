package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	"sort"
	"strings"

	//"github.com/cpflat/dot2net/pkg/clab"
	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"

	//"github.com/cpflat/dot2net/pkg/tinet"
	"github.com/cpflat/dot2net/pkg/visual"
	"github.com/urfave/cli/v2"
)

func loadContext(c *cli.Context) (d *model.Diagram, cfg *types.Config, err error) {

	dotPaths := c.Args().Slice()

	d, err = model.DiagramFromDotFile(dotPaths[0])
	if err != nil {
		return nil, nil, err
	}
	for _, dotPath := range dotPaths[1:] {
		newnd, err := model.DiagramFromDotFile(dotPath)
		if err != nil {
			return nil, nil, err
		}
		d.MergeDiagram(newnd)
	}

	cfgPath := c.String("config")
	cfg, err = types.LoadConfig(cfgPath)
	if err != nil {
		return d, cfg, err
	}

	return d, cfg, err
}

func outputString(name string, buffer []byte) error {
	if name == "" {
		fmt.Fprintln(os.Stdout, string(buffer))
	} else if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("file %v already exists", name)
	} else {
		err = os.WriteFile(name, buffer, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// func outputFiles(buffers map[string][]byte, dirname string) error {
// 	if len(buffers) == 0 {
// 		// do not generate directories for empty nodes
// 		return nil
// 	}
//
// 	if dirname == "" {
// 		for filename, buffer := range buffers {
// 			path := filename
// 			err := os.WriteFile(path, buffer, 0644)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	} else {
// 		f, err := os.Stat(dirname)
// 		if os.IsNotExist(err) {
// 			err = os.Mkdir(dirname, 0755)
// 			if err != nil {
// 				return err
// 			}
// 		} else if !f.IsDir() {
// 			return fmt.Errorf("creating directory %s fails because a file already exists", dirname)
// 		}
// 		for filename, buffer := range buffers {
// 			path := filepath.Join(dirname, filename)
// 			err = os.WriteFile(path, buffer, 0644)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }
//
// func generateBuffers(files *types.ConfigFiles) map[string][]byte {
// 	buffers := map[string][]byte{}
// 	for _, filename := range files.FileNames() {
// 		file := files.GetFile(filename)
// 		buffer := strings.Join(file.Content, "\n")
// 		buffers[filename] = []byte(buffer)
// 	}
// 	return buffers
// }

func CmdBuild(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	verbose := c.Bool("verbose")
	profile := c.String("profile")

	// init CPU profiler
	if profile != "" {
		f, err := os.Create(profile)
		if err != nil {
			return err
		}
		defer func() {
			f.Close()
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	nm, err := model.BuildNetworkModel(cfg, nd, verbose)
	if err != nil {
		return err
	}
	err = model.BuildConfigFiles(cfg, nm, verbose)
	if err != nil {
		return err
	}

	return nil
}

func CmdTinet(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	// name := c.String("output")
	verbose := c.Bool("verbose")
	profile := c.String("profile")

	// init CPU profiler
	if profile != "" {
		f, err := os.Create(profile)
		if err != nil {
			return err
		}
		defer func() {
			f.Close()
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	nm, err := model.BuildNetworkModel(cfg, nd, verbose)
	if err != nil {
		return err
	}
	err = model.BuildConfigFiles(cfg, nm, verbose)
	if err != nil {
		return err
	}

	// buffers := generateBuffers(nm.Files)
	// outputFiles(buffers, "")
	// for _, n := range nm.Nodes {
	// 	if !n.Virtual {
	// 		buffers := generateBuffers(n.Files)
	// 		outputFiles(buffers, n.Name)
	// 	}
	// }

	// spec, err := tinet.GetTinetSpecification(cfg, nm)
	// if err != nil {
	// 	return err
	// }
	// return outputString(name, spec)
	return nil
}

func CmdClab(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	// name := c.String("output")
	verbose := c.Bool("verbose")
	profile := c.String("profile")

	// init CPU profiler
	if profile != "" {
		f, err := os.Create(profile)
		if err != nil {
			return err
		}
		defer func() {
			f.Close()
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	nm, err := model.BuildNetworkModel(cfg, nd, verbose)
	if err != nil {
		return err
	}
	err = model.BuildConfigFiles(cfg, nm, verbose)
	if err != nil {
		return err
	}

	// buffers := generateBuffers(nm.Files)
	// outputFiles(buffers, "")
	// for _, n := range nm.Nodes {
	// 	if !n.Virtual {
	// 		buffers := generateBuffers(n.Files)
	// 		outputFiles(buffers, n.Name)
	// 	}
	// }

	// topo, err := clab.GetClabTopology(cfg, nm)
	// if err != nil {
	// 	return err
	// }
	// return outputString(name, topo)
	return nil
}

func CmdParams(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	name := c.String("output")
	flagall := c.Bool("all")

	nm, err := model.BuildNetworkModel(cfg, nd, false)
	if err != nil {
		return err
	}

	var numbers map[string]string
	lines := []string{}
	for _, ns := range nm.NameSpacers() {
		if flagall {
			numbers = ns.GetRelativeParams()
		} else {
			numbers = map[string]string{}
			for k, v := range ns.GetParams() {
				if k[0] != '_' {
					numbers[k] = v
				}
			}
		}
		keys := []string{}
		for k := range numbers {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, k := range keys {
			v := numbers[k]
			lines = append(lines, fmt.Sprintf("%s {{ .%+v }} = %+v", ns.StringForMessage(), k, v))
			// switch obj := ns.(type) {
			// case *types.NetworkModel:
			// 	lines = append(lines, fmt.Sprintf("network {{ .%+v }} = %+v", k, v))
			// case *types.Node:
			// 	lines = append(lines, fmt.Sprintf("%+v {{ .%+v }} = %+v", obj.Name, k, v))
			// case *types.Interface:
			// 	lines = append(lines, fmt.Sprintf("%+v.%+v {{ .%+v }} = %+v", obj.Node.Name, k, v))
			// case *types.Group:
			// 	lines = append(lines, fmt.Sprintf("%+v {{ .%+v }} = %+v", obj.Name, k, v))
			// case *types.Neighbor:
			// 	lines = append(lines, fmt.Sprintf(
			// 		"%+v.%+v (%+v-neighbor %+v.%+v) {{ .%+v }} = %+v",
			// 		node.Name, iface.Name, layer, n.Neighbor.Node.Name, n.Neighbor.Name, num, val,
			// 	))
			//
			// }
		}
	}

	// var netNumbers map[string]string
	// var nodeNumbers map[string]string
	// var ifaceNumbers map[string]string
	// var nNumbers map[string]string
	// lines := []string{}
	// if flagall {
	// 	netNumbers = nm.GetRelativeParams()
	// } else {
	// 	netNumbers = nm.GetParams()
	// }
	// keys := []string{}
	// for num := range netNumbers {
	// 	keys = append(keys, num)
	// }
	// sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	// for _, num := range keys {
	// 	val := netNumbers[num]
	// 	lines = append(lines, fmt.Sprintf("network {{ .%+v }} = %+v", num, val))
	// }

	// for _, node := range nm.Nodes {
	// 	if flagall {
	// 		nodeNumbers = node.GetRelativeParams()
	// 	} else {
	// 		nodeNumbers = node.GetParams()
	// 	}

	// 	keys := []string{}
	// 	for num := range nodeNumbers {
	// 		keys = append(keys, num)
	// 	}
	// 	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	// 	for _, num := range keys {
	// 		val := nodeNumbers[num]
	// 		lines = append(lines, fmt.Sprintf("%+v {{ .%+v }} = %+v", node.Name, num, val))
	// 	}

	// 	for _, iface := range node.Interfaces {
	// 		if flagall {
	// 			ifaceNumbers = iface.GetRelativeParams()
	// 		} else {
	// 			ifaceNumbers = iface.GetParams()
	// 		}

	// 		keys := []string{}
	// 		for num := range ifaceNumbers {
	// 			keys = append(keys, num)
	// 		}
	// 		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	// 		for _, num := range keys {
	// 			val := ifaceNumbers[num]
	// 			lines = append(lines, fmt.Sprintf("%+v.%+v {{ .%+v }} = %+v", node.Name, iface.Name, num, val))
	// 		}

	// 		if flagall {
	// 			for layer, neighbors := range iface.Neighbors {
	// 				for _, n := range neighbors {
	// 					nNumbers = n.GetRelativeParams()
	// 					keys := []string{}
	// 					for num := range nNumbers {
	// 						keys = append(keys, num)
	// 					}
	// 					sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	// 					for _, num := range keys {
	// 						val := nNumbers[num]
	// 						lines = append(lines, fmt.Sprintf(
	// 							"%+v.%+v (%+v-neighbor %+v.%+v) {{ .%+v }} = %+v",
	// 							node.Name, iface.Name, layer, n.Neighbor.Node.Name, n.Neighbor.Name, num, val,
	// 						))
	// 					}
	// 				}
	// 			}
	// 			for _, m := range iface.GetMembers() {
	// 				mNumbers := m.GetRelativeParams()
	// 				keys := []string{}
	// 				for num := range mNumbers {
	// 					keys = append(keys, num)
	// 				}
	// 				sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	// 				for _, num := range keys {
	// 					val := mNumbers[num]
	// 					line := fmt.Sprintf(
	// 						"%v.%v (%v %v member %v) {{ .%v }} = %v",
	// 						node.Name, iface.Name, m.ClassType, m.ClassName, m.Member, num, val)
	// 					lines = append(lines, line)
	// 				}
	// 			}
	// 		}
	// 	}

	// 	for _, group := range node.Groups {
	// 		keys := []string{}
	// 		for num := range group.GetParams() {
	// 			keys = append(keys, num)
	// 		}
	// 		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	// 		for _, num := range keys {
	// 			val, err := group.GetParamValue(num)
	// 			if err != nil {
	// 				return err
	// 			}
	// 			lines = append(lines, fmt.Sprintf("%+v.%+v {{ .%+v }} = %+v", node.Name, group.Name, num, val))
	// 		}
	// 	}

	// }

	buf := strings.Join(lines, "\n")
	err = outputString(name, []byte(buf))
	return err
}

func CmdVisual(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	name := c.String("output")
	layer := c.String("layer")

	nm, err := model.BuildNetworkModel(cfg, nd, false)
	if err != nil {
		return err
	}

	buf, err := visual.GraphToDot(cfg, nm, layer)
	if err != nil {
		return err
	}
	err = outputString(name, []byte(buf))
	return err
}

func CmdData(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	name := c.String("output")

	nm, err := model.BuildNetworkModel(cfg, nd, false)
	if err != nil {
		return err
	}

	buf, err := visual.GetDataJSON(cfg, nm)
	if err != nil {
		return err
	}
	err = outputString(name, []byte(buf))
	return err
}
