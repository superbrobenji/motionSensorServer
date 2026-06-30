package mesh

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type CommandStatus string

const (
	CommandStatusPending CommandStatus = "pending"
	CommandStatusAcked   CommandStatus = "acked"
	CommandStatusTimeout CommandStatus = "timeout"
)

type PendingCommand struct {
	ID      string
	NodeID  uint8
	Action  string
	SentAt  time.Time
	Status  CommandStatus
	AckedAt *time.Time
}

type CommandStore struct {
	mu       sync.RWMutex
	commands map[string]*PendingCommand
}

func NewCommandStore() *CommandStore {
	return &CommandStore{commands: make(map[string]*PendingCommand)}
}

func (cs *CommandStore) Add(cmd *PendingCommand) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.commands[cmd.ID] = cmd
}

func (cs *CommandStore) Ack(commandID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cmd, ok := cs.commands[commandID]
	if !ok {
		return false
	}
	now := time.Now()
	cmd.Status = CommandStatusAcked
	cmd.AckedAt = &now
	return true
}

func (cs *CommandStore) Get(commandID string) (*PendingCommand, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	cmd, ok := cs.commands[commandID]
	if !ok {
		return nil, false
	}
	c := *cmd
	return &c, true
}

// AckByToken finds a pending command whose UUID bytes 14 and 15 match the
// 2-byte correlation token embedded in OP_COMMAND_ACK frames, marks it acked,
// and returns its full command ID.
func (cs *CommandStore) AckByToken(token [2]byte) (string, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for id, cmd := range cs.commands {
		if cmd.Status != CommandStatusPending {
			continue
		}
		u, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		if u[14] == token[0] && u[15] == token[1] {
			now := time.Now()
			cmd.Status = CommandStatusAcked
			cmd.AckedAt = &now
			return id, true
		}
	}
	return "", false
}
