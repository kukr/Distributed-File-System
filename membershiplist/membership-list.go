package membershiplist

import (
	"fmt"
	"log"
	"sync"
	"time"
	"net"
	"strings"

	globob "cs425/mp/glob"
)

type NodeStatus int

// Define timeouts
var (
	T_FAIL   = 1 // 2 second
	T_DELETE = 1 // 2 second
	T_LEAVE  = 2 // 2 second
)

// Enum for Node status
const (
	Alive NodeStatus = iota
	Suspicious
	Failed
	Delete
	Left
)

type messageType uint8

const (
	MessageUpdate      messageType = 1
	MessageJoin        messageType = 2
	MessageFailed      messageType = 3
	MessageDelete 	   messageType = 4
)

// Type of Membership list
type MembershipList struct {
	sync.RWMutex
	items []MembershipListItem
}

// Type of NodeState
type NodeState struct {
	Status    NodeStatus
	Timestamp time.Time
}

// Type of each item in the Membership list
type MembershipListItem struct {
	Id                string
	State             NodeState
	IncarnationNumber int
	UDPPort           int
}

/**
* Overriding the String method for the type MembershipListItem
 */
func (memListItem MembershipListItem) String() string {
	return fmt.Sprintf("[%s || %v ||  %d\n]", memListItem.Id, memListItem.State, memListItem.IncarnationNumber)
}

/**
* Overriding the String method for the type NodeState
 */
func (nodeState NodeState) String() string {
	return fmt.Sprintf("%v at %v", nodeState.Status, nodeState.Timestamp)
}

/**
* Overriding the String method for the type NodeStatus
 */
func (nodeStatus NodeStatus) String() string {
	switch nodeStatus {
	case Alive:
		return "Alive"
	case Suspicious:
		return "Suspicious"
	case Failed:
		return "Failed"
	case Delete:
		return "Delete"
	case Left:
		return "Left"
	default:
		return fmt.Sprintf("%d", int(nodeStatus))
	}
}

/**
* Create a new instance of a thread safe Membership list
 */
func NewMembershipList(listItems ...([]MembershipListItem)) *MembershipList {
	var cs *MembershipList
	if len(listItems) == 0 {
		cs = &MembershipList{
			items: make([]MembershipListItem, 0),
		}
	} else {
		cs = &MembershipList{
			items: listItems[0],
		}
	}

	return cs
}

/**
* Appends an item to the membership list
 */
func (ml *MembershipList) Append(item MembershipListItem) int {
	ml.Lock()
	defer ml.Unlock()

	ml.items = append(ml.items, item)

	indexOfInsertedItem := len(ml.items) - 1

	return indexOfInsertedItem
}

/**
* Iterates over the items in the membership list
* Each item is sent over a channel, so that
* we can iterate over the slice using the builin range keyword
 */
func (ml *MembershipList) Iter() <-chan MembershipListItem {
	c := make(chan MembershipListItem)

	f := func() {
		ml.RLock()
		defer ml.RUnlock()
		for _, value := range ml.items {
			c <- value
		}
		close(c)
	}
	go f()

	return c
}

/**
* Method to get length of MembershipList
 */
func (ml *MembershipList) Len() int {
	ml.RLock()
	defer ml.RUnlock()

	return len(ml.items)
}

/**
* Method to get the membership list
 */
func (ml *MembershipList) GetList() []MembershipListItem {
	return ml.items
}

/**
* Overriding the String method of the type MembershipList
 */
func (ml *MembershipList) String() string {
	return fmt.Sprintf("%v", ml.items)
}

/**
* Method to clean the MembershipList
* Accessed by the stabilisation protocol
 */
func (ml *MembershipList) Clean() {
	log.Printf("Cleaning Membership list")
	ml.Lock()
	defer ml.Unlock()
	// Deleting processes that Failed and the ones that Left voluntarily
	log.Printf("Old MembershipList: \n%v", ml.items)
	var newItems []MembershipListItem
	for _, item := range ml.items {
		if item.State.Status != Delete {
			newItems = append(newItems, item)
		} else {
			log.Printf("Deleting: %v", item)
		}
	}

	ml.items = newItems

	log.Printf("New MembershipList: \n%v", ml.items)
}

/**
* Method to update the Incarnation number of the process
 */
func (ml *MembershipList) UpdateSelfIncarnationNumber(myId string) {
	ml.Lock()
	defer ml.Unlock()

	log.Printf("Updating my (%v) Incarnation Number\n", myId)
	for i := 0; i < len(ml.items); i++ {
		if ml.items[i].Id == myId {
			ml.items[i].IncarnationNumber = ml.items[i].IncarnationNumber + 1
			break
		}

	}
}

/**
* Method to mark a process as Suspicious in the Membership List
 */
func (ml *MembershipList) MarkSus(nodeId string) {
	ml.Lock()
	defer ml.Unlock()

	log.Printf("Marking node (%v) Suspicious\n", nodeId)
	for i := 0; i < len(ml.items); i++ {
		if ml.items[i].Id == nodeId && ml.items[i].State.Status == Alive {
			ml.items[i].State.Status = Suspicious
			ml.items[i].State.Timestamp = time.Now()
			break
		}

	}
}

/**
* Method to mark the node as wanting to leave
 */
func (ml *MembershipList) MarkLeave(nodeId string) {
	ml.Lock()
	defer ml.Unlock()

	log.Printf("Marking node (%v) as Left\n", nodeId)
	for i := 0; i < len(ml.items); i++ {
		if ml.items[i].Id == nodeId {
			ml.items[i].State.Status = Left
			ml.items[i].State.Timestamp = time.Now()
			break
		}

	}
}

/**
* Method that merges a new Membership list with itself
 */
func (m1 *MembershipList) Merge(m2 []MembershipListItem) {
	m1.Lock()
	defer m1.Unlock()

	log.Printf("Merging Membership lists\n")
	log.Printf("Current Membership List\n%v\v\n", m1.items)
	log.Printf("Received Membership List\n%v\v\n", m2)

	var newItems []MembershipListItem

	i := 0
	j := 0

	for i < len(m1.items) && j < len(m2) {
		m1Item := m1.items[i]
		m2Item := m2[j]

		if m1Item.Id == m2Item.Id {
			// States Failed, Delete and Left in the current list to be maintained
			if m1Item.State.Status == Failed || m1Item.State.Status == Delete || m1Item.State.Status == Left {
				newItems = append(newItems, m1Item)
			} else {
				// From the incoming list, Failed, Delete and Left States are given prime importance
				if (m2Item.State.Status == Failed || m2Item.State.Status == Delete || m2Item.State.Status == Left) &&
					(m1Item.State.Status != m2Item.State.Status) {
					m1Item.State = m2Item.State
					m1Item.State.Timestamp = time.Now()
					newItems = append(newItems, m1Item)
				} else {
					// For other states, update based on increased incarnation number
					if m2Item.IncarnationNumber > m1Item.IncarnationNumber {
						m1Item.State = m2Item.State
						m1Item.State.Timestamp = time.Now()
						m1Item.IncarnationNumber = m2Item.IncarnationNumber
						newItems = append(newItems, m1Item)
					} else {
						newItems = append(newItems, m1Item)
					}
				}
			}
			i++
			j++
		} else {
			j++
		}
	}
	// Current list has more itemss
	for i < len(m1.items) {
		newItems = append(newItems, m1.items[i])
		i++
	}

	// There are new processes that need to be added to current membership list
	for j < len(m2) {
		if m2[j].State.Status == Alive || m2[j].State.Status == Suspicious {
			m2[j].State.Timestamp = time.Now()
			newItems = append(newItems, m2[j])
		}
		j++
	}

	m1.items = newItems

	log.Printf("New Membership List: \n%v\n\n", newItems)
}

func generateFailedBuffer(hostIp string) []byte {
	replyBuf := []byte{byte(MessageFailed)}
	hostIp = (strings.Split(hostIp, ":"))[0]
	hostName := globob.IptoHostMap[hostIp]
	log.Printf("HostName wala : %s \n", hostName)
	replyBuf = append(replyBuf, ':')
	replyBuf = append(replyBuf, []byte(hostName)...)
	return replyBuf
}

func GetthisHostIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("Couldn't get the IP address of the process\n%v", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
/**
* Update the states of the items in the membership list
* Run by the stabilisation protocol
 */
func (ml *MembershipList) UpdateStates() []MembershipListItem {
	log.Printf("Updating states of processes in Membership List\n")
	ml.Lock()
	defer ml.Unlock()

	currentTime := time.Now().Unix()

	for i := 0; i < len(ml.items); i++ {
		if ml.items[i].State.Status == Suspicious &&
			(currentTime-ml.items[i].State.Timestamp.Unix() >= int64(T_FAIL)) {
			// If the process is Suspicious for more than T_FAIL seconds, mark it as failed
			ml.items[i].State.Status = Failed

			//failedMsg := generateFailedBuffer(ml.items[i].Id)
			log.Printf("Host detected as failed: %s", ml.items[i].Id)
			
			thisHostIp := GetthisHostIP()

			thisHostAddrwithPort := fmt.Sprintf("%s:%d", thisHostIp, globob.SDFS_PORT)
			conn, err := net.Dial("udp", thisHostAddrwithPort)

			if err != nil {
				log.Println("Update States 1st error: ", err)
			}

			failedMsg := generateFailedBuffer(ml.items[i].Id)

			_, err = conn.Write(failedMsg)

			if err != nil {
				log.Println("update States 2nd error: ", err)
			}

			conn.Close()

		} else if ml.items[i].State.Status == Failed &&
			(currentTime-ml.items[i].State.Timestamp.Unix() >= int64(T_DELETE)) {
			// If the process is in the Failed state for more than T_DELETE seconds,
			//  move it to Delete state
			ml.items[i].State.Status = Delete

		} else if ml.items[i].State.Status == Left &&
			(currentTime-ml.items[i].State.Timestamp.Unix() >= int64(T_LEAVE)) {
			// If the process is in the Left state for more than T_LEAVE seconds, mark it to be deleted
			ml.items[i].State.Status = Delete
		}
	}

	return ml.items

}
