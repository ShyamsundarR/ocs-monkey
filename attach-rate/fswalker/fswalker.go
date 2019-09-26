package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	//"k8s.io/kubernetes/pkg/util/mount"
)

var (
	parent = flag.String("parent", "/testdir", "Base directory for traversal (defaults to \"/testdir\"")
	group  = flag.Int64("group", 853254, "FSGroup to set to (defaults to 853254")
)

const (
	rwMask   = os.FileMode(0660)
	roMask   = os.FileMode(0440)
	execMask = os.FileMode(0110)
)

// SetVolumeOwnership modifies the given volume to be owned by
// fsGroup, and sets SetGid so that newly created files are owned by
// fsGroup. If fsGroup is nil nothing is done.
func SetVolumeOwnership(parentPath string, fsGroup *int64) error {

	//dummyMount := mount.New("")

	if fsGroup == nil {
		return nil
	}

	return filepath.Walk(parentPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil
		}

		if stat == nil {
			fmt.Printf("Got nil stat_t for path %v while setting ownership of volume\n", path)
			return nil
		}

		err = os.Chown(path, int(stat.Uid), int(*fsGroup))
		if err != nil {
			fmt.Printf("Chown failed on %v: %v\n", path, err)
		}

		mask := rwMask
		//dummyMount.GetSELinuxSupport(parentPath)

		if info.IsDir() {
			mask |= os.ModeSetgid
			mask |= execMask
		}

		err = os.Chmod(path, info.Mode()|mask)
		if err != nil {
			fmt.Printf("Chmod failed on %v: %v\n", path, err)
		}

		return nil
	})
}

func main() {
	flag.Parse()

	err := SetVolumeOwnership(*parent, group)
	if err != nil {
		fmt.Printf("Traversal failed on %v: %v", *parent, err)
	}
}
