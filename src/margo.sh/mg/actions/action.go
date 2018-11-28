package actions

import (
	"github.com/ugorji/go/codec"
)

// Action is an object that's dispatched to update an agent's state.
type Action interface {
	actionType()

	// ActionLabel returns the name of an action for display in the editor ui.
	ActionLabel() string
}

// ActionType is the base implementation of an action.
type ActionType struct{}

func (ActionType) actionType() {}

// ActionLabel implemented Action.ActionLabel().
func (ActionType) ActionLabel() string { return "" }

// ActionData is data coming from the client.
type ActionData struct {
	// Name is the name of the action
	Name string

	// Data is the raw encoded data of the action
	Data codec.Raw

	// Handle is the handle to use for decoding Data
	Handle codec.Handle
}

// Decode decodes the encoded data into the action pointer p.
func (d ActionData) Decode(p interface{}) error {
	return codec.NewDecoderBytes(d.Data, d.Handle).Decode(p)
}

// ClientAction is an action that may be sent to the client.
type ClientAction interface {
	ClientAction() ClientData
}

// ClientData is the marshal-able form of a ClientAction.
type ClientData struct {
	Name string
	Data interface{}
}
