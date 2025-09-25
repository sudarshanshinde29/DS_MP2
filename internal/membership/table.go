package membership

import (
	"fmt"
	"sync"
	"time"

	mpb "DS_MP2/protoBuilds/membership"
)

type Member struct {
	NodeID      *mpb.NodeID
	State       mpb.MemberState
	Incarnation uint64
	LastUpdate  time.Time
}

type Table struct {
	mu      sync.RWMutex //thread safety - making sure multiple goroutinescan safely access the same data without causing problems.
	self    *mpb.NodeID
	members map[string]*Member // key: StringifyNodeID(node)
	logger  func(string, ...interface{})
}

func NewTable(self *mpb.NodeID, logger func(string, ...interface{})) *Table {
	// Create a new table
	t := &Table{
		self:    self,
		members: make(map[string]*Member),
		logger:  logger,
	}
	// Add self as ALIVE
	t.addSelf()
	return t
}

// function to add self to the table
func (t *Table) addSelf() {
	key := StringifyNodeID(t.self)
	t.members[key] = &Member{
		NodeID:      t.self,
		State:       mpb.MemberState_ALIVE,
		Incarnation: t.self.GetIncarnation(),
		LastUpdate:  time.Now(),
	}
	t.logger("Added self: %s", key)
}

// ApplyUpdate processes a membership entry
func (t *Table) ApplyUpdate(entry *mpb.MembershipEntry) bool {
	if entry == nil || entry.Node == nil {
		return false
	}

	key := StringifyNodeID(entry.Node)
	now := time.Now()

	t.mu.Lock()         // Lock for writing
	defer t.mu.Unlock() // Unlock when function exits

	existing, exists := t.members[key]

	// If new entry, add it
	if !exists {
		t.members[key] = &Member{
			NodeID:      entry.Node,
			State:       entry.State,
			Incarnation: entry.Incarnation,
			LastUpdate:  now,
		}
		t.logger("Added new member: %s state=%v", key, entry.State)
		return true
	}

	// just update state and timestamp
	existing.State = entry.State
	existing.Incarnation = entry.Incarnation
	existing.LastUpdate = now
	t.logger("Updated member: %s state=%v", key, entry.State)
	return true
}

func (t *Table) GetMembers() []*Member {
	t.mu.RLock()         // Lock for reading
	defer t.mu.RUnlock() // Unlock when function exits

	result := make([]*Member, 0, len(t.members))
	for _, member := range t.members {
		result = append(result, &Member{
			NodeID:      member.NodeID,
			State:       member.State,
			Incarnation: member.Incarnation,
			LastUpdate:  member.LastUpdate,
		})
	}
	return result
}

func (t *Table) GetSelf() *mpb.NodeID {
	return t.self
}

func (t *Table) String() string {
	t.mu.RLock()         // Lock for reading
	defer t.mu.RUnlock() // Unlock when function exits

	result := fmt.Sprintf("Membership Table (%d members):\n", len(t.members))
	for key, member := range t.members {
		result += fmt.Sprintf("  %s: state=%v incarnation=%d last_update=%v\n",
			key, member.State, member.Incarnation, member.LastUpdate)
	}
	return result
}
