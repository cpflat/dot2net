package kathara

import (
	"embed"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

const ModuleName = "kathara"

const KatharaOutputFile = "lab.conf"

// StartupFile is how Kathara lets a lab act on a device once it is up. The name
// is the key config entries reference; the file itself is <device>.startup and
// sits beside lab.conf, which is where Kathara looks for it.
const StartupFile = "kathara_startup"
const StartupFileSuffix = ".startup"

const VolumeParamRuleName = "kathara_volumes"
const CopyParamRuleName = "kathara_copies"
const ScriptFile = "kathara.sh"

// InterfaceNamePrefix is not a default but a requirement. Kathara names a
// device's interfaces after the index written in lab.conf - r1[0] becomes eth0
// inside the container - and offers no way to change that, so a topology asking
// for another prefix cannot be honoured. Rather than overwrite the request in
// silence, the module reports it.
const InterfaceNamePrefix = "eth"

// Parameters the module attaches to each interface. The collision domain is the
// shared medium an interface sits on; the index is the N of r1[N], which has to
// match the ethN the container ends up with.
const CollisionDomainParamName = "_kathara_cd"
const InterfaceIndexParamName = "_kathara_index"

// WorkerGroupClassName carries lab.conf when the topology declares placement
// units. Not types.WorkerGroupClassName: that is the name a topology writes in
// its DOT file, and this is the module's own class, applied to the worker groups
// ClassifyObjects picks out.
const WorkerGroupClassName = "_katharaWorkerGroup"

const NetworkClassName = "_katharaNetwork"
const NodeClassName = "_katharaNode"
const InterfaceClassName = "_katharaInterface"

const KatharaLineFormatName = "_katharaLine"

// KatharaCopyFormatName ends the copy commands with a newline so that the
// topology's own startup commands begin on a line of their own. A startup file
// is a shell script, so a newline left at the end when there are no startup
// commands costs nothing.
const KatharaCopyFormatName = "_katharaCopy"

// Kathara validates these itself and fails the whole lab, so the same rules are
// checked here where the message can name the topology's own object.
var deviceNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,30}$`)
var collisionDomainPattern = regexp.MustCompile(`^\w+$`)

//go:embed templates/*
var templates embed.FS

type KatharaModule struct {
	*types.StandardModule
}

// Capabilities provided by this module.
var (
	_ types.Module             = (*KatharaModule)(nil)
	_ types.ObjectClassifier   = (*KatharaModule)(nil)
	_ types.ParameterProvider  = (*KatharaModule)(nil)
	_ types.RequirementChecker = (*KatharaModule)(nil)
	_ types.ParameterGenerator = (*KatharaModule)(nil)
)

func NewModule() types.Module {
	return &KatharaModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *KatharaModule) UpdateConfig(cfg *types.Config) error {
	// Every line of lab.conf stands on its own: the file is a set of
	// declarations, not a sequence, and Kathara's parser reads one line at a
	// time without caring about their order.
	cfg.AddFormatStyle(&types.FormatStyle{
		Name:                KatharaLineFormatName,
		MergeBlockSeparator: "\n",
	})
	cfg.AddFormatStyle(&types.FormatStyle{
		Name:                KatharaCopyFormatName,
		FormatBlockSuffix:   "\n",
		MergeBlockSeparator: "\n",
	})

	// The lab lives in a directory of its own, and not only for tidiness:
	// Kathara reads a directory named after a device, next to lab.conf, as
	// files to copy into that device once it has started. The nodes' generated
	// files sit at the output root under exactly such names, so a lab.conf
	// beside them would set that copying off - a second delivery nobody asked
	// for, running after the device is up. With the lab one level down, the
	// convention finds nothing and the only way in is the one the topology
	// chose.
	// One lab.conf, or one per machine. A lab is deployed to a single machine,
	// so a topology spread over several needs a file each - the same choice
	// containerlab and TiNET make, and for the same reason. What can be read at
	// this point is the topology's own configuration, which is loaded before the
	// modules are.
	_, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName)
	scope := types.ClassTypeNetwork
	if perWorker {
		scope = types.ClassTypeGroup
	}
	cfg.AddFileDefinition(&types.FileDefinition{
		Name:   KatharaOutputFile,
		Path:   "",
		Scope:  scope,
		Subdir: ModuleName,
	})

	// The template text is the same either way: it aggregates over "the devices
	// of this object", and the object differs. A device's volume paths are
	// relative to the lab directory, so ../r1/etc/frr means this machine's r1
	// once the lab sits under that machine's own directory.
	ct, err := templateFrom(cfg, "templates/lab.conf.network", &types.ConfigTemplate{
		File: KatharaOutputFile,
	})
	if err != nil {
		return err
	}
	labOwned := []*types.ConfigTemplate{ct}

	var opts Options
	if _, err := cfg.DecodeModuleConfig(ModuleName, &opts); err != nil {
		return err
	}
	// The entry script joins the same class rather than getting one of its own:
	// a class of its own is a class no group carries a label for, and the script
	// would silently not be written for a multi-machine lab.
	if opts.GenerateScripts {
		entry, err := entryScriptTemplate(cfg, scope)
		if err != nil {
			return err
		}
		labOwned = append(labOwned, entry)
	}

	if perWorker {
		// Not AddModuleGroupClassLabel: that would give lab.conf to every group,
		// including the ones that only share parameters. ClassifyObjects picks
		// the worker groups out.
		cfg.AddGroupClass(&types.GroupClass{
			Name:            WorkerGroupClassName,
			ConfigTemplates: labOwned,
		})
	} else {
		cfg.AddNetworkClass(&types.NetworkClass{
			Name:            NetworkClassName,
			ConfigTemplates: labOwned,
		})
	}

	ct, err = templateFrom(cfg, "templates/lab.conf.node_kathara_device", &types.ConfigTemplate{
		Name:          "kathara_device",
		Format:        KatharaLineFormatName,
		Depends:       []string{"kathara_image", "kathara_volumes"},
		PlatformEntry: true,
	})
	if err != nil {
		return err
	}
	// Kathara starts a device and then copies its files in, so anything the
	// image reads while booting is read before those files exist: an FRR
	// container comes up with the image's own daemons file, not the topology's.
	// The startup file is what runs after the copy, which is why the startup
	// commands a topology already writes for the other platforms are carried
	// here rather than being left out.
	// The filename carries the prefix too: Kathara reads <device>.startup, and
	// the device is what the prefix renamed.
	cfg.AddFileDefinition(&types.FileDefinition{
		Name:       StartupFile,
		NamePrefix: cfg.NodeNamePrefix(),
		NameSuffix: StartupFileSuffix,
		Scope:      types.ClassTypeNode,
		Output:     "root",
		Subdir:     ModuleName,
	})
	// containerlab and TiNET both refuse a topology without a startup template;
	// Kathara has no reason to, so the dependency is named only when there is
	// one to depend on.
	var startupDepends []string
	for _, nc := range cfg.NodeClasses {
		for _, t := range nc.ConfigTemplates {
			if t.Name == "startup" {
				startupDepends = []string{"startup"}
			}
		}
	}
	// The copies come first: a command the author wrote may use a file that is
	// only there once it has been copied.
	ctCopies, err := templateFrom(cfg, "templates/startup.node_kathara_copies", &types.ConfigTemplate{
		Name:           "kathara_copies",
		Format:         KatharaCopyFormatName,
		RequiredParams: []string{"values_kathara_copy_entry"},
	})
	if err != nil {
		return err
	}
	// What the file holds is worked out first, so that the file itself can ask
	// whether anything came of it: either the copies or the topology's startup
	// commands are reason enough to write it, and neither alone can say so.
	ctStartupBody, err := templateFrom(cfg, "templates/startup.node_kathara_startup_body", &types.ConfigTemplate{
		Name:    "kathara_startup_body",
		Depends: append([]string{"kathara_copies"}, startupDepends...),
	})
	if err != nil {
		return err
	}
	// The file is written for every device, even when there is nothing to put
	// in it: Kathara reads an empty startup file without complaint, and a file
	// that appears only sometimes is one that dot2net files cannot promise.
	ctStartup, err := templateFrom(cfg, "templates/startup.node_kathara_startup", &types.ConfigTemplate{
		Name:    "kathara_startup",
		File:    StartupFile,
		Depends: []string{"kathara_startup_body"},
	})
	if err != nil {
		return err
	}

	// The image is a device option like any other, and Kathara falls back to its
	// own base image when none is given - which is why a topology that names one
	// has to have it carried through, or the lab comes up without the software
	// it was written for. RequiredParams leaves the line out when the topology
	// names no image, so a device may still take the default on purpose.
	ctImage, err := templateFrom(cfg, "templates/lab.conf.node_kathara_image", &types.ConfigTemplate{
		Name:           "kathara_image",
		Format:         KatharaLineFormatName,
		RequiredParams: []string{"image"},
		PlatformEntry:  true,
	})
	if err != nil {
		return err
	}
	// Kathara copies a device's files in after it has started, so a file the
	// software reads while booting arrives too late. A volume is mounted before
	// the device starts, which is what the other platforms' bind mounts do, so
	// the files are put in place that way instead.
	ctVolumes, err := templateFrom(cfg, "templates/lab.conf.node_kathara_volumes", &types.ConfigTemplate{
		Name:           "kathara_volumes",
		Format:         KatharaLineFormatName,
		RequiredParams: []string{"values_kathara_volume_entry"},
	})
	if err != nil {
		return err
	}
	// What the entry script needs from each node: the lab's own teardown
	// commands, and the files to copy out. Both are aggregated by the script,
	// which is the module's own file - a topology never names these blocks.
	ctTeardown, err := readEntryTemplate("templates/teardown.node_kathara_teardown", &types.ConfigTemplate{
		Name:           "kathara_teardown",
		Depends:        []string{"teardown"},
		RequiredParams: []string{"self_teardown"},
	})
	if err != nil {
		return err
	}
	ctCollect, err := readEntryTemplate("templates/collect.node_kathara_collect", &types.ConfigTemplate{
		Name:           "kathara_collect",
		RequiredParams: []string{"values_kathara_collect_entry"},
	})
	if err != nil {
		return err
	}
	collectEntry, err := templates.ReadFile("templates/collect.value_kathara_collect_entry")
	if err != nil {
		return err
	}
	cfg.AddParameterRule(&types.ParameterRule{
		Name:      "kathara_collects",
		Mode:      types.ParameterRuleModeAttach,
		Generator: "kathara.collectfiles",
		ConfigTemplates: []*types.ConfigTemplate{
			{Name: "kathara_collect_entry", Template: []string{string(collectEntry)}},
		},
	})

	cfg.AddNodeClass(&types.NodeClass{
		Name:            NodeClassName,
		Parameters:      []string{VolumeParamRuleName, CopyParamRuleName, "kathara_collects"},
		ConfigTemplates: []*types.ConfigTemplate{ct, ctImage, ctStartup, ctStartupBody, ctCopies, ctVolumes, ctTeardown, ctCollect},
	})

	entry, err := templateFrom(cfg, "templates/lab.conf.value_kathara_volume_entry", &types.ConfigTemplate{
		Name: "kathara_volume_entry",
	})
	if err != nil {
		return err
	}
	copyEntry, err := templateFrom(cfg, "templates/startup.value_kathara_copy_entry", &types.ConfigTemplate{
		Name: "kathara_copy_entry",
	})
	if err != nil {
		return err
	}
	cfg.AddParameterRule(&types.ParameterRule{
		Name:            CopyParamRuleName,
		Mode:            types.ParameterRuleModeAttach,
		Generator:       ModuleName + ".copyfiles",
		ConfigTemplates: []*types.ConfigTemplate{copyEntry},
	})

	cfg.AddParameterRule(&types.ParameterRule{
		Name:            VolumeParamRuleName,
		Mode:            types.ParameterRuleModeAttach,
		Generator:       ModuleName + ".filemounts",
		ConfigTemplates: []*types.ConfigTemplate{entry},
	})

	// RequiredLink: the line declares a wire, so it must not be emitted for a
	// connection that models a shared segment without an actual link.
	ct, err = templateFrom(cfg, "templates/lab.conf.interface_kathara_interface", &types.ConfigTemplate{
		Name:         "kathara_interface",
		Format:       KatharaLineFormatName,
		RequiredLink: true,
	})
	if err != nil {
		return err
	}
	cfg.AddInterfaceClass(&types.InterfaceClass{
		Name:            InterfaceClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct},
	})

	return nil
}

// templateFrom reads a template and fills in the lab's node name prefix. Kathara
// builds a container's name out of the user, the lab and the device, and gives
// it a "name" label holding the device alone - so the device's name is where one
// lab is told apart from another. The prefix is empty unless this run was given
// a lab name, leaving a topology generated the usual way exactly as it was.
func templateFrom(cfg *types.Config, path string, ct *types.ConfigTemplate) (*types.ConfigTemplate, error) {
	bytes, err := templates.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ct.Template = []string{strings.ReplaceAll(string(bytes), "%%NODEPREFIX%%", cfg.NodeNamePrefix())}
	return ct, nil
}

// ClassifyObjects gives the lab.conf lines to the devices and interfaces that
// end up in the file. A switch node is not a device: Kathara realizes a shared
// medium as a collision domain, which exists only as the name its members
// mention, so the node itself has no line of its own.
func (m *KatharaModule) ClassifyObjects(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			continue
		}
		node.AddModuleClassLabels(NodeClassName)
		for _, iface := range node.Interfaces {
			iface.AddModuleClassLabels(InterfaceClassName)
		}
	}
	// A lab.conf per machine when the topology places its nodes on several. Only
	// the worker groups get it: a group that exists to share parameters is not a
	// machine, and nothing is deployed to it.
	for _, group := range nm.Groups {
		if cfg.IsWorkerGroup(group) {
			group.AddModuleClassLabels(WorkerGroupClassName)
		}
	}
	return nil
}

// GenerateParameters names the collision domain each interface sits on, and the
// index it takes in its device's line.
//
// A connection to a switch names the switch: everything meeting there shares one
// medium, which is exactly what a collision domain is. A connection between two
// ordinary devices names the connection, giving it a medium of its own.
func (m *KatharaModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			continue
		}
		for _, iface := range node.Interfaces {
			// Only the interfaces that reach lab.conf: the ones the platform
			// wires. Kathara names an interface after its index in that line, so
			// it has nothing to say about an interface it never lists - a bridge
			// the node builds for itself keeps whatever name the topology chose.
			if iface.DeployForm() != types.DeployLink || iface.Connection == nil {
				continue
			}
			cd := iface.Connection.Name
			if iface.Opposite != nil && cfg.IsSwitchNode(iface.Opposite.Node) {
				cd = iface.Opposite.Node.Name
			}
			if !collisionDomainPattern.MatchString(cd) {
				return fmt.Errorf(
					"collision domain name %q is not accepted by Kathara (it allows only word characters)", cd)
			}
			iface.AddParam(CollisionDomainParamName, cd)

			index, err := interfaceIndex(iface)
			if err != nil {
				return err
			}
			iface.AddParam(InterfaceIndexParamName, strconv.Itoa(index))
		}
	}
	return nil
}

// interfaceIndex recovers the N of ethN. The name is where the number lives:
// Kathara derives the interface name inside the container from the index in
// lab.conf, so the two cannot be chosen independently.
func interfaceIndex(iface *types.Interface) (int, error) {
	rest, ok := strings.CutPrefix(iface.Name, InterfaceNamePrefix)
	if !ok {
		return 0, fmt.Errorf(
			"interface %s of node %s is not named %sN, so its index in lab.conf cannot be told",
			iface.Name, iface.Node.Name, InterfaceNamePrefix)
	}
	index, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf(
			"interface %s of node %s is not named %sN, so its index in lab.conf cannot be told",
			iface.Name, iface.Node.Name, InterfaceNamePrefix)
	}
	return index, nil
}

func (m *KatharaModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {

	if err := checkMountDirsUsed(cfg, nm); err != nil {
		return err
	}

	for _, node := range nm.Nodes {
		if !node.IsMaterialised() || cfg.IsSwitchNode(node) {
			continue
		}
		if !deviceNamePattern.MatchString(node.Name) {
			return fmt.Errorf(
				"node name %q is not accepted by Kathara (lowercase letters, digits and "+
					"underscores, at most 30 characters)", node.Name)
		}
		// Kathara rejects a device whose interface indexes have a hole, and the
		// indexes come from the names. Automatic naming hands the numbers to the
		// wired interfaces first, so it cannot leave one; a hole means the
		// topology chose the names itself and skipped a number.
		seen := map[int]bool{}
		count := 0
		for _, iface := range node.Interfaces {
			// The same set GenerateParameters walks: the interfaces that get a
			// line in lab.conf, and so a name Kathara decides.
			if iface.DeployForm() != types.DeployLink {
				continue
			}
			index, err := interfaceIndex(iface)
			if err != nil {
				return err
			}
			seen[index] = true
			count++
		}
		for i := 0; i < count; i++ {
			if !seen[i] {
				return fmt.Errorf(
					"node %s has a hole in its interface indexes (%s%d is missing), which Kathara "+
						"rejects: it gives an interface the name its index in lab.conf earns, so the "+
						"indexes have to run from 0 without a gap. Let the interfaces be named "+
						"automatically, or name them so that the numbers are consecutive",
					node.Name, InterfaceNamePrefix, i)
			}
		}
	}
	return nil
}

// Options are the Kathara module's own settings, written under
// module_config.kathara.
type Options struct {
	// MountDirs names the directories inside a device that dot2net supplies
	// entirely. Kathara mounts a directory rather than a file, and the mount
	// replaces what the image had there, so this says which directories the
	// topology is willing to take over.
	MountDirs []string `yaml:"mount_dirs"`

	// GenerateScripts writes an entry point script beside the lab. It carries
	// that Kathara reads its files from the directory it runs in, and that
	// kathara exec cannot pass a command containing -c.
	GenerateScripts bool `yaml:"generate_scripts"`
}

// addEntryScript registers the script that stands in front of Kathara's own
// commands. Kathara is not split by GlobalSettings.SplitModuleOutput: its lab
// is the directory itself, holding lab.conf and every <device>.startup, and
// those startup files are written by the topology rather than by this module.
func entryScriptTemplate(cfg *types.Config, scope string) (*types.ConfigTemplate, error) {
	cfg.AddFileDefinition(&types.FileDefinition{
		Name:       ScriptFile,
		Path:       "",
		Scope:      scope,
		Subdir:     ModuleName,
		Executable: true,
	})
	bytes, err := templates.ReadFile("templates/kathara.sh.entry")
	if err != nil {
		return nil, err
	}
	return &types.ConfigTemplate{File: ScriptFile, Template: []string{katharaScript(cfg, string(bytes))}}, nil
}

// katharaScript fills in what the entry script cannot know until it is written:
// where collected files go, and the prefix that tells this lab's devices from
// another's.
func katharaScript(cfg *types.Config, script string) string {
	script = strings.ReplaceAll(script, "%%COLLECT%%", "../"+types.CollectDirName)
	return strings.ReplaceAll(script, "%%NODEPREFIX%%", cfg.NodeNamePrefix())
}

// GenerateValueParameters implements types.ParameterGenerator.
func (m *KatharaModule) GenerateValueParameters(
	generatorName string,
	target types.ValueOwner,
	cfg *types.Config,
	nm *types.NetworkModel,
) ([]map[string]string, error) {
	switch generatorName {
	case "filemounts":
		return m.generateFilemountParams(target, cfg)
	case "copyfiles":
		return copyFileParams(target, cfg)
	case "collectfiles":
		return generateCollectParams(target, cfg, nm)
	default:
		return nil, fmt.Errorf("unknown generator: %s", generatorName)
	}
}

// generateFilemountParams names the directories to mount into a device.
//
// Kathara mounts directories and refuses single files, so the unit is not the
// file but a directory the topology has declared as its own in
// module_config.kathara.mount_dirs. That declaration is not a restatement of
// what the paths already say: mounting a directory replaces the image's own, so
// everything the image kept there is hidden, and dot2net cannot see inside an
// image to know whether that is safe. A file whose directory was not declared
// is reported rather than quietly delivered some other way.
func (m *KatharaModule) generateFilemountParams(
	target types.ValueOwner,
	cfg *types.Config,
) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("filemounts generator requires Node target, got %T", target)
	}
	// A node whose configuration is withheld has no files to deliver, so naming
	// one would point at a path that does not exist.
	if !node.IsMaterialised() || node.IsVirtual() || cfg.IsSwitchNode(node) {
		return nil, nil
	}

	var opts Options
	if _, err := cfg.DecodeModuleConfig(ModuleName, &opts); err != nil {
		return nil, err
	}

	generated := make(map[string]bool)
	for _, name := range node.FilesToGenerate(cfg) {
		generated[name] = true
	}

	mounted := make(map[string]bool)
	var results []map[string]string
	for _, fileDef := range cfg.FileDefinitions {
		if fileDef.Path == "" || !generated[fileDef.Name] {
			continue
		}
		// A file provided by copy is not mounted at its own path: it waits in
		// the staging directory, mounted below.
		if fileDef.GetProvide() == types.ProvideCopy {
			continue
		}

		dir, ok := owningDir(opts.MountDirs, fileDef.Path)
		if !ok {
			return nil, fmt.Errorf(
				"file %s is to be mounted at %s, but Kathara mounts directories rather than files, "+
					"and %s is not among module_config.kathara.mount_dirs. Add the directory there if "+
					"dot2net supplies everything the software needs in it - mounting it hides what the "+
					"image kept there - or give the file provide: copy, which places it once the device "+
					"has started",
				fileDef.Name, fileDef.Path, path.Dir(fileDef.Path))
		}
		if mounted[dir] {
			continue
		}
		mounted[dir] = true

		results = append(results, map[string]string{
			"device": cfg.NodeNamePrefix() + node.Name,
			"source": path.Join("..", node.Name, strings.TrimPrefix(dir, "/")),
			"target": dir,
		})
	}

	stagingDir, staged, err := node.StagingDir(cfg)
	if err != nil {
		return nil, err
	}
	if staged {
		// The staging directory is dot2net's own and exists in no image, so it
		// hides nothing and needs no declaration.
		results = append(results, map[string]string{
			"device": cfg.NodeNamePrefix() + node.Name,
			"source": path.Join("..", stagingDir),
			"target": "/" + types.StagingDirName,
		})
	}
	return results, nil
}

// owningDir returns the declared directory that holds the given path.
func owningDir(mountDirs []string, filePath string) (string, bool) {
	for _, dir := range mountDirs {
		clean := path.Clean(dir)
		if path.Dir(filePath) == clean || strings.HasPrefix(filePath, clean+"/") {
			return clean, true
		}
	}
	return "", false
}

// copyFileParams turns the node's staged files into the source and target a copy
// command names. The command itself lives in the module's template, since only
// the platform knows where its commands are written.
func copyFileParams(target types.ValueOwner, cfg *types.Config) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("copyfiles generator requires Node target, got %T", target)
	}
	// A node whose configuration is withheld has no files to deliver, so naming
	// one would point at a path that does not exist.
	if !node.IsMaterialised() || node.IsVirtual() || cfg.IsSwitchNode(node) {
		return nil, nil
	}
	copies, err := types.StagedCopies(cfg, node)
	if err != nil {
		return nil, err
	}
	var results []map[string]string
	for _, c := range copies {
		results = append(results, map[string]string{
			"source": c.Source,
			"target": c.Target,
			"dir":    c.Dir,
		})
	}
	return results, nil
}

// checkMountDirsUsed rejects a declared directory that holds none of the
// generated files. Taking over a directory of a device's filesystem is not a
// thing to do by accident, and a name that matches nothing is nearly always a
// misspelling of one that would have.
func checkMountDirsUsed(cfg *types.Config, nm *types.NetworkModel) error {
	var opts Options
	if _, err := cfg.DecodeModuleConfig(ModuleName, &opts); err != nil {
		return err
	}
	if len(opts.MountDirs) == 0 {
		return nil
	}

	used := make(map[string]bool, len(opts.MountDirs))
	for _, node := range nm.Nodes {
		if !node.IsMaterialised() || cfg.IsSwitchNode(node) {
			continue
		}
		generated := make(map[string]bool)
		for _, name := range node.FilesToGenerate(cfg) {
			generated[name] = true
		}
		for _, filedef := range cfg.FileDefinitions {
			if filedef.Path == "" || !generated[filedef.Name] {
				continue
			}
			if filedef.GetProvide() != types.ProvideMount {
				continue
			}
			if dir, ok := owningDir(opts.MountDirs, filedef.Path); ok {
				used[dir] = true
			}
		}
	}

	for _, dir := range opts.MountDirs {
		if !used[path.Clean(dir)] {
			return fmt.Errorf(
				"module_config.kathara.mount_dirs names %s, but no file is generated into it; "+
					"a directory is mounted so that the files below it reach the device, so this "+
					"one either is misspelled or is left over", dir)
		}
	}
	return nil
}

// readEntryTemplate fills a config template in from the module's own files.
func readEntryTemplate(path string, ct *types.ConfigTemplate) (*types.ConfigTemplate, error) {
	bytes, err := templates.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ct.Template = []string{string(bytes)}
	return ct, nil
}

// generateCollectParams names the files to copy out of a node.
func generateCollectParams(target types.ValueOwner, cfg *types.Config, nm *types.NetworkModel) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("collectfiles generator requires Node target, got %T", target)
	}
	targets, err := types.CollectTargets(cfg, nm)
	if err != nil {
		return nil, err
	}
	var results []map[string]string
	for _, t := range targets {
		if t.Node != node.Name {
			continue
		}
		results = append(results, map[string]string{"device": t.Node, "path": t.ContainerPath})
	}
	return results, nil
}
