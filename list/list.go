package list

import (
	"unsafe"
)

// Head represents a doubly linked list node with prev and next pointers.
// It forms a circular doubly linked list structure commonly used in Linux kernel.
// The len field tracks the number of elements in the list when this Head is used as the list head.
type Head struct {
	Next *Head // Pointer to the next node in the list
	Prev *Head // Pointer to the previous node in the list
	len  int   // Length counter (only used by list head, not individual nodes)
}

// Init initializes the list head to point to itself, forming a circular empty list.
// After initialization, both Next and Prev point to the head itself.
// The length is reset to 0.
func (h *Head) Init() {
	h.Next = h
	h.Prev = h
	h.len = 0
}

// Add inserts a new node at the head of the list (after the list head).
// This method increments the list length counter.
// The new node is inserted between the current head and its next node.
func (h *Head) Add(new *Head) {
	h.len++
	add(new, h, h.Next)
}

// AddTail inserts a new node at the tail of the list (before the list head).
// This method increments the list length counter.
// The new node is inserted between the current tail (h.Prev) and the head.
func (h *Head) AddTail(new *Head) {
	h.len++
	add(new, h.Prev, h)
}

// Del removes a node from the list.
// This API could be designed as func (h *Head) Del(), but to manage the list's
// length counter properly, it's designed to be called on the list head with
// the node to delete as parameter.
// This method decrements the list length counter.
func (h *Head) Del(head *Head) {
	h.len--
	del(head.Prev, head.Next)
}

// Entry retrieves the containing structure pointer from the list node.
// It uses pointer arithmetic to calculate the address of the containing structure
// given the offset of the Head field within that structure.
// offset: the byte offset of the Head field in the containing structure
// Returns: unsafe pointer to the containing structure
func (h *Head) Entry(offset uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(unsafe.Pointer(h)) - offset)
}

// FirstEntry returns the first element in the list when called on the list head.
// It's equivalent to calling NextEntry on the head.
func (h *Head) FirstEntry(offset uintptr) unsafe.Pointer {
	return h.NextEntry(offset)
}

// LastEntry returns the last element in the list when called on the list head.
// It's equivalent to calling PrevEntry on the head.
func (h *Head) LastEntry(offset uintptr) unsafe.Pointer {
	return h.PrevEntry(offset)
}

// FirstEntryOrNil returns the first element in the list, or nil if the list is empty.
// This is a safe way to get the first element without checking emptiness first.
func (h *Head) FirstEntryOrNil(offset uintptr) unsafe.Pointer {
	if h.len == 0 {
		return nil
	}
	return h.FirstEntry(offset)
}

// NextEntry retrieves the next element's containing structure pointer.
// It calculates the structure pointer for the next node in the list.
func (h *Head) NextEntry(offset uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(unsafe.Pointer(h.Next)) - offset)
}

// PrevEntry retrieves the previous element's containing structure pointer.
// It calculates the structure pointer for the previous node in the list.
func (h *Head) PrevEntry(offset uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(unsafe.Pointer(h.Prev)) - offset)
}

// ForEach iterates forward through the list, calling the callback for each node.
// WARNING: This method is NOT safe for list modifications during iteration.
// If you need to modify the list during iteration, use ForEachSafe instead.
func (h *Head) ForEach(callback func(pos *Head)) {
	for pos := h.Next; pos != h; pos = pos.Next {
		callback(pos)
	}
}

// ForEachPrev iterates backward through the list, calling the callback for each node.
// WARNING: This method is NOT safe for list modifications during iteration.
// If you need to modify the list during iteration, use ForEachPrevSafe instead.
func (h *Head) ForEachPrev(callback func(pos *Head)) {
	for pos := h.Prev; pos != h; pos = pos.Prev {
		callback(pos)
	}
}

// ForEachSafe iterates forward through the list safely.
// This version saves the next pointer before calling the callback,
// making it safe to delete or move the current node during iteration.
func (h *Head) ForEachSafe(callback func(pos *Head)) {
	pos := h.Next
	n := pos.Next

	for pos != h {
		callback(pos)
		pos = n
		n = pos.Next
	}
}

// ForEachPrevSafe iterates backward through the list safely.
// This version saves the previous pointer before calling the callback,
// making it safe to delete or move the current node during iteration.
func (h *Head) ForEachPrevSafe(callback func(pos *Head)) {
	pos := h.Prev
	n := pos.Prev
	for pos != h {
		callback(pos)
		pos = n
		n = pos.Prev
	}
}

// Len returns the current length of the list.
// Only meaningful when called on the list head.
func (h *Head) Len() int {
	return h.len
}

// Replace replaces an old list head with a new one.
// The new head takes over all nodes from the old head.
// The length counter is also transferred to the new head.
func (h *Head) Replace(new *Head) {
	old := h
	new.Next = old.Next
	new.Next.Prev = new
	new.Prev = old.Prev
	new.Prev.Next = new
	new.len = old.len
}

// ReplaceInit replaces the old list head with a new one and reinitializes the old head.
// After this operation, the old head becomes an empty circular list.
func (h *Head) ReplaceInit(new *Head) {
	h.Replace(new)
	h.Init()
}

// DelInit deletes a node from the list and reinitializes it.
// The deleted node becomes an empty circular list pointing to itself.
// This method decrements the list length counter.
func (h *Head) DelInit(pos *Head) {
	h.len--
	delEntry(pos)
	pos.Init()
}

// Move removes a node from its current position and inserts it at the head of this list.
// This is equivalent to deleting from the old list and adding to the new list head.
func (h *Head) Move(list *Head) {
	delEntry(list)
	h.Add(list)
}

// MoveTail removes a node from its current position and inserts it at the tail of this list.
// This is equivalent to deleting from the old list and adding to the new list tail.
func (h *Head) MoveTail(list *Head) {
	delEntry(list)
	h.AddTail(list)
}

// IsLast checks if this node is the last node in the list.
// Returns true if the next pointer points to the head (circular list property).
func (h *Head) IsLast() bool {
	return h.Next == h
}

// Empty checks if the list is empty.
// The list is considered empty when the length is 0.
func (h *Head) Empty() bool {
	return h.len == 0
}

// RotateLeft rotates the list to the left by one position.
// The first element becomes the last element.
// If the list is empty, this operation does nothing.
func (h *Head) RotateLeft() {
	var first *Head
	if !h.Empty() {
		first = h.Next
		h.MoveTail(first)
	}
}

// del is an internal helper function that removes a node from the list.
// It updates the prev and next pointers to bypass the deleted node.
// prev: the node before the one being deleted
// next: the node after the one being deleted
func del(prev *Head, next *Head) {
	next.Prev = prev
	prev.Next = next
}

// add is an internal helper function that inserts a new node into the list.
// It updates all necessary pointers to maintain the circular doubly linked list structure.
// new: the node to insert
// prev: the node that will be before the new node
// next: the node that will be after the new node
func add(new, prev, next *Head) {
	next.Prev = new
	new.Next = next
	new.Prev = prev
	prev.Next = new
}

// delEntry is an internal helper function that removes a node from the list
// by calling del with the node's prev and next pointers.
func delEntry(entry *Head) {
	del(entry.Prev, entry.Next)
}
