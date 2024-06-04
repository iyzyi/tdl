package iyzyi

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type _tupleFromID struct {
	_type  string // download or forward
	fromID int64
}

func tupleFromID(_type string, fromID int64) _tupleFromID {
	return _tupleFromID{_type: _type, fromID: fromID}
}

type _tupleMsgID struct {
	_tupleFromID
	msgID int
}

func tupleMsgID(_type string, fromID int64, msgID int) _tupleMsgID {
	return _tupleMsgID{
		_tupleFromID: tupleFromID(_type, fromID),
		msgID:        msgID,
	}
}

type Recorder struct {
	recordFiles map[_tupleFromID]*os.File
	msgIDs      map[_tupleMsgID]struct{}
	recordDir   string
}

func NewRecorder() (r *Recorder, err error) {
	r = &Recorder{
		recordFiles: make(map[_tupleFromID]*os.File),
		msgIDs:      make(map[_tupleMsgID]struct{}),
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path, error: %v\n", err)
		return
	}
	execDir := filepath.Dir(execPath)
	r.recordDir = filepath.Join(execDir, "record")

	parse := func(_type string) (err error) {
		recordDir := filepath.Join(r.recordDir, _type)
		err = os.MkdirAll(recordDir, os.ModePerm)
		if err != nil {
			fmt.Printf("Failed to create %v, error: %v\n", recordDir, err)
			return
		}

		files, err := filepath.Glob(filepath.Join(recordDir, "*.txt"))
		for _, filePath := range files {
			fileName := filepath.Base(filePath)
			fileNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			var fromID int64
			fromID, err = strconv.ParseInt(fileNameNoExt, 10, 64)
			if err != nil {
				fmt.Printf("Failed to convert file name to fromID, error: %v\n", err)
				return
			}

			var file *os.File
			file, err = os.OpenFile(filePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				fmt.Printf("Failed to open %v, error: %v\n", filePath, err)
				return
			}
			r.recordFiles[tupleFromID(_type, fromID)] = file

			reader := bufio.NewReader(file)
			var line string
			var msgID int
			for {
				line, err = reader.ReadString('\n')
				if err != nil {
					if err.Error() == "EOF" {
						err = nil
						break
					}
					fmt.Printf("Failed to read the record file, error: %v\n", err)
					return
				}
				msgID, err = strconv.Atoi(line[:len(line)-1])
				if err != nil {
					fmt.Printf("Failed to parse integer of the record file, error: %v", err)
					return
				}
				r.msgIDs[tupleMsgID(_type, fromID, msgID)] = struct{}{}
			}
		}
		return
	}

	err = parse("download")
	if err != nil {
		return
	}

	err = parse("forward")
	if err != nil {
		return
	}

	return r, nil
}

func (r *Recorder) Recorded(_type string, fromID int64, msgID int) (err error) {
	file, ok := r.recordFiles[tupleFromID(_type, fromID)]
	if !ok {
		filePath := filepath.Join(r.recordDir, _type, fmt.Sprintf("%v.txt", fromID))
		file, err = os.OpenFile(filePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			fmt.Printf("Failed to open %v, error: %v\n", filePath, err)
			return
		}
		r.recordFiles[tupleFromID(_type, fromID)] = file
	}

	line := fmt.Sprintf("%v\n", msgID)
	_, err = file.WriteString(line)
	if err != nil {
		fmt.Printf("Failed to write to the record file, error: %v", err)
	}
	return
}

func (r *Recorder) IsRecorded(_type string, fromID int64, msgID int) bool {
	_, ok := r.msgIDs[tupleMsgID(_type, fromID, msgID)]
	return ok
}
