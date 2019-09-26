/* Server end of the network workload generator */
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	testdir   = flag.String("testdir", "/testdir", "Base directory for creating test data (defaults to \"/testdir\"")
	dirdepth  = flag.Int("dirdepth", 5, "Depth of directories to create (defaults to 5)")
	dircount  = flag.Int("dircount", 100, "Count of directories to create per dirdepth (defaults to 100)")
	filecount = flag.Int("filecount", 100, "Count of files to create per directory (defaults to 100)")
	tsr       = flag.Bool("tsr", false, "Terminate and stay resident (defaults to false)")
)

// TestState, strongly typed JSON for stashing requirements of state
type TestState struct {
	DirDepth   int  `json:"DirDepth"`
	DirCount   int  `json:"DirCount"`
	FileCount  int  `json:"FileCount"`
	InProgress bool `json:"InProgress"`
	TestDir    string
}

// file name in which test state metadata is stashed
const testStateFileName = "teststate-meta.json"

// stashTestState stashes test state into testStateFileName under path, in JSON format
func stashTestState(testState *TestState, path string) error {
	encodedBytes, err := json.Marshal(testState)
	if err != nil {
		return fmt.Errorf("failed to marshall JSON for state:(%+v) with error (%v)", testState, err)
	}

	fPath := filepath.Join(path, testStateFileName)
	err = ioutil.WriteFile(fPath, encodedBytes, 0600)
	if err != nil {
		return fmt.Errorf("failed to stash state (%+v) with error (%v) at path (%s)",
			testState, err, fPath)
	}

	return nil
}

// lookupTestStateStash reads and returns stashed test state at passed in path
func lookupTestStateStash(path string) (TestState, error) {
	var testState TestState

	testState.TestDir = path
	testState.InProgress = false

	fPath := filepath.Join(path, testStateFileName)
	encodedBytes, err := ioutil.ReadFile(fPath) // #nosec - intended reading from fPath
	if err != nil {
		if !os.IsNotExist(err) {
			return testState, fmt.Errorf("failed to read stashed test state from path (%s): (%v)", fPath, err)
		}

		return testState, nil
	}

	err = json.Unmarshal(encodedBytes, &testState)
	if err != nil {
		return testState, fmt.Errorf("failed to unmarshall stashed JSON test state from path (%s): (%v)", fPath, err)
	}

	return testState, nil
}

// cleanupTestStateStash cleans up any stashed test state at passed in path
func cleanupTestStateStash(path string) error {
	fPath := filepath.Join(path, testStateFileName)
	if err := os.Remove(fPath); err != nil {
		return fmt.Errorf("failed to cleanup stashed test state data (%s): (%v)", fPath, err)
	}

	return nil
}

func CreateDirectories(mydepth int, newDir bool, myParent string, onDiskState, desiredState TestState) error {
	startIdx := onDiskState.DirCount + 1
	endIdx := desiredState.DirCount

	if newDir {
		startIdx = 1
	}

	fmt.Printf("(LOG) CreateDirectories in %s\n", myParent+"/dir_"+strconv.Itoa(mydepth)+"_["+strconv.Itoa(startIdx)+".."+strconv.Itoa(endIdx)+"]")
	for dirIdx := startIdx; dirIdx <= endIdx; dirIdx++ {
		dName := myParent + "/dir_" + strconv.Itoa(mydepth) + "_" + strconv.Itoa(dirIdx)
		err := os.MkdirAll(dName, os.ModePerm)
		if err != nil {
			fmt.Printf("(ERROR) Failed creating directory %s (%v)\n", dName, err)
			return err
		}
	}

	return nil
}

func CreateFiles(myParent string, startIdx, endIdx int) error {
	fmt.Printf("(LOG) CreateFiles in %s\n", myParent+"/file"+"_["+strconv.Itoa(startIdx)+".."+strconv.Itoa(endIdx)+"]")
	for fileIdx := startIdx; fileIdx <= endIdx; fileIdx++ {
		fName := myParent + "/file_" + strconv.Itoa(fileIdx)
		tFile, err := os.Create(fName)
		if err != nil {
			fmt.Printf("(ERROR) Failed creating file %s (%v)\n", fName, err)
			return err
		}
		tFile.Close()
	}

	return nil
}

func CreateContent(mydepth, mydirindex int, newDir bool, myParent string, onDiskState, desiredState TestState) error {
	if mydepth > onDiskState.DirDepth {
		newDir = true
	}

	if onDiskState.DirCount < desiredState.DirCount || newDir {
		err := CreateDirectories(mydepth, newDir, myParent, onDiskState, desiredState)
		if err != nil {
			fmt.Printf("(ERROR) Failed creating directories %v\n", err)
			return err
		}
	}

	// update file count on older directories, at older depths
	dirIdx := 1
	if !newDir {
		if onDiskState.FileCount < desiredState.FileCount {
			for dirIdx = 1; dirIdx <= onDiskState.DirCount; dirIdx++ {
				dirPath := myParent + "/dir_" + strconv.Itoa(mydepth) + "_" + strconv.Itoa(dirIdx)
				err := CreateFiles(dirPath, onDiskState.FileCount+1, desiredState.FileCount)
				if err != nil {
					fmt.Printf("(ERROR) Failed creating files (%v) in dir %s\n", err, dirPath)
					return err
				}
			}
		} else if mydirindex < onDiskState.DirCount {
			dirIdx = onDiskState.DirCount + 1
		}
	}

	// update file count on newer directories
	for ; dirIdx <= desiredState.DirCount; dirIdx++ {
		dirPath := myParent + "/dir_" + strconv.Itoa(mydepth) + "_" + strconv.Itoa(dirIdx)
		err := CreateFiles(dirPath, 1, desiredState.FileCount)
		if err != nil {
			fmt.Printf("(ERROR) Failed creating files (%v) in dir %s\n", err, dirPath)
			return err
		}
	}

	// For each directory move to the next depth
	if mydepth < desiredState.DirDepth {
		for dirIdx = 1; dirIdx <= desiredState.DirCount; dirIdx++ {
			dirPath := myParent + "/dir_" + strconv.Itoa(mydepth) + "_" + strconv.Itoa(dirIdx)
			newDir = newDir || dirIdx >= onDiskState.DirCount+1
			err := CreateContent(mydepth+1, dirIdx, newDir, dirPath, onDiskState, desiredState)
			if err != nil {
				fmt.Printf("(ERROR) Content creation (%v) in dir %s at depth %d\n", err, dirPath, mydepth+1)
				return err
			}
		}
	}

	return nil
}

func UpdateDiskContents(onDiskState, desiredState TestState) error {
	err := CreateContent(1, 0, false, desiredState.TestDir+"/testdata", onDiskState, desiredState)
	if err != nil {
		fmt.Printf("(ERROR) Failed creating content %v\n", err)
		return err
	}

	return nil
}

func tsrOrExit(tsr bool, errCode int) {
	if tsr {
		time.Sleep(100 * time.Hour)
	}

	os.Exit(errCode)
}

func main() {
	var desiredState TestState

	flag.Parse()

	// Load current state of test
	onDiskState, err := lookupTestStateStash(*testdir)
	if err != nil {
		fmt.Printf("(ERROR) Unable to determine existing test state %v\n", err)
		tsrOrExit(*tsr, -1)
	}

	// If on disk state is in progress, it denotes an unclean exit from prior runs, bail out!
	if onDiskState.InProgress {
		fmt.Printf("(ERROR) Found an earlier on disk state in progress, cannot recover actual state\n")
		tsrOrExit(*tsr, -1)
	}

	// Setup desired state of the test
	desiredState.TestDir = *testdir
	desiredState.DirDepth = *dirdepth
	desiredState.DirCount = *dircount
	desiredState.FileCount = *filecount
	desiredState.InProgress = true

	// Check if desire state is a shrink (not handling shrink yet!)
	if onDiskState.DirDepth > desiredState.DirDepth ||
		onDiskState.DirCount > desiredState.DirCount ||
		onDiskState.FileCount > desiredState.FileCount {
		fmt.Printf("(ERROR) Desired state of test is lower than on disk contents, canot shrink!\n")
		tsrOrExit(*tsr, -1)
	}

	// Check if any work needs to be done
	if onDiskState.DirDepth == desiredState.DirDepth &&
		onDiskState.DirCount == desiredState.DirCount &&
		onDiskState.FileCount == desiredState.FileCount {
		fmt.Printf("(SUCCESS) Desired state determined as current state of disk contents\n")
		tsrOrExit(*tsr, 0)
	}

	// Update on disk state as in progress
	err = stashTestState(&desiredState, desiredState.TestDir)
	if err != nil {
		fmt.Printf("(ERROR) Failed to stash desired state on disk (%v)\n", err)
		tsrOrExit(*tsr, -1)
	}

	// do the required work
	err = UpdateDiskContents(onDiskState, desiredState)
	if err != nil {
		fmt.Printf("(ERROR) Failed to update disk contents and reach desired state (%v)\n", err)
		tsrOrExit(*tsr, -1)
	}

	// Update on disk state as done
	desiredState.InProgress = false
	err = stashTestState(&desiredState, desiredState.TestDir)
	if err != nil {
		fmt.Printf("(ERROR) Failed to stash final state on disk (%v)\n", err)
		tsrOrExit(*tsr, -1)
	}

	tsrOrExit(*tsr, 0)
}
