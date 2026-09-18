package transfer

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"

	"github.com/schollz/pake/v3"
)

// The room code is a weak secret: about 5.9 million combinations (22.5
// bits), small enough to guess offline if it ever leaked in a form an
// attacker could test against. PAKE (password-authenticated key exchange)
// turns it into a strong shared key without putting the code — or anything
// derived from it that could be attacked offline — on the wire.
//
// What this buys us concretely:
//
//   - The rendezvous server stops being a trusted party. It is the server
//     that tells the receiver "the sender is peer X"; if it lied and named
//     one of its own peers instead, the receiver used to connect to the
//     impostor and accept whatever files it offered. Now the impostor has
//     to know the room code to complete the handshake, and it does not.
//   - A passive listener on either link learns nothing it can test codes
//     against.
//
// What it does not buy: protection from someone who guesses the code
// outright. The code is also what the server looks rooms up by, so a
// lookup that hits *is* a correct guess. The defence against guessing is
// the rate at which the server answers lookups (see rendezvous.Registry)
// and, behind a tunnel, the per-address limit the tunnel enforces.
//
// The exchange binds both peer IDs into the session key, so an attacker
// cannot sit in the middle and pass the two sides' messages through to each
// other: the two ends would derive different keys and the confirmation step
// below would fail.
//
// On the choice of library: schollz/pake is a SPAKE2-style exchange of its
// own design, not RFC 9382 SPAKE2 or CPace, and has had far less review
// than either. It is what croc has used in production for years, and the
// parts that carry the weight here — binding both identities, and the
// mutual confirmation tags below — are ours and do not depend on its
// details. Moving to a standard construction when a maintained Go
// implementation exists is a change to this file and the protocol
// version, nothing else.
const (
	// pakeCurve is the group the exchange runs in. P-256 comes from the
	// standard library, where it is constant-time and well reviewed.
	pakeCurve = "p256"

	// Roles in the underlying protocol. The receiver dials and therefore
	// speaks first, which makes it party A.
	roleReceiver = 0
	roleSender   = 1

	// Domain separators for the two confirmation tags, so a tag from one
	// side can never be replayed as the other's.
	confirmReceiverLabel = "puresend/pake/confirm/receiver/v1"
	confirmSenderLabel   = "puresend/pake/confirm/sender/v1"
)

// Credentials are the three things both sides must already agree on before
// a single file byte moves: the room code, and who the two ends are. The
// sender learns the receiver's peer ID from the incoming stream; the
// receiver learns the sender's from the room lookup. If those two views
// disagree — which is exactly what a lying server or a peer in the middle
// produces — the handshake fails.
type Credentials struct {
	Code     string // the room code, e.g. "kiraz-liman-42"
	Sender   string // peer ID of the side serving the files
	Receiver string // peer ID of the side asking for them
}

func (c Credentials) validate() error {
	switch {
	case c.Code == "":
		return fmt.Errorf("room code is required")
	case c.Sender == "":
		return fmt.Errorf("sender identity is required")
	case c.Receiver == "":
		return fmt.Errorf("receiver identity is required")
	}
	return nil
}

// identities returns the ordered identity strings bound into the session
// key. Party A is the receiver, party B the sender; both sides must build
// this the same way or the keys will not match.
func (c Credentials) identities() (idA, idB []byte) {
	return []byte("puresend/receiver/" + c.Receiver),
		[]byte("puresend/sender/" + c.Sender)
}

// authMsg is one step of the handshake. Exactly one field is set.
type authMsg struct {
	PAKE    []byte `json:"pake,omitempty"`    // a marshalled PAKE public value
	Confirm []byte `json:"confirm,omitempty"` // a key confirmation tag
}

// readMsgFunc and writeMsgFunc hide the fact that the two sides frame
// messages differently: the sender can decode straight off the stream,
// while the receiver has to read bounded lines because raw file bytes
// follow on the same stream.
type (
	readMsgFunc  func(*authMsg) error
	writeMsgFunc func(authMsg) error
)

// authenticate runs the exchange and returns the session key. Four
// messages: the two PAKE halves, then a confirmation tag from each side.
//
// The confirmation is not optional. The exchange alone leaves both sides
// with *a* key; only exchanging tags over it proves the other side derived
// the same one, which is what makes a wrong code fail loudly instead of
// silently corrupting the transfer that follows.
func authenticate(role int, creds Credentials, read readMsgFunc, write writeMsgFunc) ([]byte, error) {
	if err := creds.validate(); err != nil {
		return nil, err
	}
	idA, idB := creds.identities()
	p, err := pake.InitCurveWithIdentities([]byte(creds.Code), role, pakeCurve, idA, idB)
	if err != nil {
		return nil, fmt.Errorf("could not start the security handshake: %w", err)
	}

	if role == roleReceiver {
		if err := write(authMsg{PAKE: p.Bytes()}); err != nil {
			return nil, fmt.Errorf("could not start the security handshake: %w", err)
		}
	}

	var theirs authMsg
	if err := read(&theirs); err != nil {
		return nil, fmt.Errorf("no answer to the security handshake: %w", describe(err))
	}
	if len(theirs.PAKE) == 0 {
		return nil, ErrWrongCode
	}
	if err := p.Update(theirs.PAKE); err != nil {
		// A malformed or off-curve value means the other side is not
		// running this protocol, or is not who it claims to be.
		return nil, ErrWrongCode
	}

	if role == roleSender {
		if err := write(authMsg{PAKE: p.Bytes()}); err != nil {
			return nil, fmt.Errorf("could not answer the security handshake: %w", err)
		}
	}

	key, err := p.SessionKey()
	if err != nil {
		return nil, ErrWrongCode
	}

	// Mutual confirmation. Each side sends the tag for its own role and
	// checks the other's, in the same order as the exchange above so that
	// neither side is ever writing while the other is also writing.
	ours, expected := confirmSenderLabel, confirmReceiverLabel
	if role == roleReceiver {
		ours, expected = confirmReceiverLabel, confirmSenderLabel
	}
	send := func() error {
		return write(authMsg{Confirm: confirmTag(key, ours)})
	}
	check := func() error {
		var msg authMsg
		if err := read(&msg); err != nil {
			return fmt.Errorf("no confirmation from the other side: %w", describe(err))
		}
		if !hmac.Equal(msg.Confirm, confirmTag(key, expected)) {
			return ErrWrongCode
		}
		return nil
	}

	if role == roleReceiver {
		if err := send(); err != nil {
			return nil, err
		}
		if err := check(); err != nil {
			return nil, err
		}
		return key, nil
	}

	// The sender answers even when the check failed. Hanging up instead
	// would leave the receiver reading an empty connection, and it would
	// report a dropped link when the real answer is "that is not the code".
	failed := check()
	if err := send(); err != nil && failed == nil {
		return nil, err
	}
	if failed != nil {
		return nil, failed
	}
	return key, nil
}

// confirmTag is the proof that a side holds the session key.
func confirmTag(key []byte, label string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}
