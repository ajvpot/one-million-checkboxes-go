package state

import (
	"encoding/gob"
	"log"
	"math"
	"os"
	"sync/atomic"
)

const (
	NumCheckboxes = 1000000
	stateFilePath = "checkboxes_state.gob"
)

func init() {
	if NumCheckboxes > math.MaxUint32/2 {
		panic("Number of checkboxes is too large for bit packing hack")
	}
}

var (
	checkboxes [NumCheckboxes]atomic.Bool
)

// SaveStateToFile saves the current state of checkboxes to a file.
func SaveStateToFile() {
	file, err := os.Create(stateFilePath)
	if err != nil {
		log.Println("Error creating state file:", err)
		return
	}
	defer file.Close()

	state := make([]bool, NumCheckboxes)
	for i := 0; i < NumCheckboxes; i++ {
		state[i] = checkboxes[i].Load()
	}

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(state); err != nil {
		log.Println("Error encoding state to file:", err)
	}
}

// LoadStateFromFile loads the state of checkboxes from a file.
func LoadStateFromFile() {
	file, err := os.Open(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("State file does not exist, starting with empty state")
			return
		}
		log.Println("Error opening state file:", err)
		return
	}
	defer file.Close()

	var state []bool
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		log.Println("Error decoding state from file:", err)
		return
	}

	for i := 0; i < NumCheckboxes && i < len(state); i++ {
		checkboxes[i].Store(state[i])
	}
}

// UpdateCheckbox updates the state of a checkbox at a given index.
func UpdateCheckbox(index uint32, value bool) {
	if index >= NumCheckboxes {
		log.Println("Invalid checkbox index")
		return
	}
	checkboxes[index].Store(value)
}

// GetCheckboxState returns the state of a checkbox at a given index.
func GetCheckboxState(index uint32) bool {
	if index >= NumCheckboxes {
		log.Println("Invalid checkbox index")
		return false
	}
	return checkboxes[index].Load()
}
