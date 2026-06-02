package channel

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrChallengeCapacity = errors.New("challenge store capacity exceeded")

const maxChallenges = 10000

type Challenge struct {
	ID               string
	Platform         string
	AppKey           string
	ExpectedUserHash string
	ReturnTo         string
	ExpiresAt        time.Time
}

type State struct {
	ID          string
	ChallengeID string
	ExpiresAt   time.Time
}

// ChallengeStore keeps short-lived OAuth binding challenges and states.
type ChallengeStore struct {
	mu         sync.Mutex
	challenges map[string]Challenge
	states     map[string]State
	ttl        time.Duration
}

func NewChallengeStore(ttl time.Duration) *ChallengeStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &ChallengeStore{
		challenges: make(map[string]Challenge),
		states:     make(map[string]State),
		ttl:        ttl,
	}
}

func (s *ChallengeStore) Create(platform, appKey, expectedUserHash, returnTo string) (Challenge, error) {
	if platform == "" || appKey == "" || expectedUserHash == "" {
		return Challenge{}, fmt.Errorf("platform, app_key, and expected user hash are required")
	}
	id, err := RandomToken("ch_", 24)
	if err != nil {
		return Challenge{}, err
	}
	ch := Challenge{
		ID:               id,
		Platform:         platform,
		AppKey:           appKey,
		ExpectedUserHash: expectedUserHash,
		ReturnTo:         returnTo,
		ExpiresAt:        time.Now().Add(s.ttl),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	if len(s.challenges)+len(s.states) >= maxChallenges {
		return Challenge{}, ErrChallengeCapacity
	}
	s.challenges[id] = ch
	return ch, nil
}

func (s *ChallengeStore) CreateState(challengeID string) (State, error) {
	id, err := RandomToken("st_", 24)
	if err != nil {
		return State{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)
	if len(s.challenges)+len(s.states) >= maxChallenges {
		return State{}, ErrChallengeCapacity
	}
	ch, ok := s.challenges[challengeID]
	if !ok || ch.ExpiresAt.Before(now) {
		return State{}, fmt.Errorf("challenge expired or not found")
	}
	state := State{ID: id, ChallengeID: challengeID, ExpiresAt: now.Add(s.ttl)}
	s.states[id] = state
	return state, nil
}

func (s *ChallengeStore) Peek(challengeID string) (Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)
	ch, ok := s.challenges[challengeID]
	if !ok || ch.ExpiresAt.Before(now) {
		return Challenge{}, fmt.Errorf("challenge expired or not found")
	}
	return ch, nil
}

func (s *ChallengeStore) TakeState(stateID string) (Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)

	state, ok := s.states[stateID]
	if !ok || state.ExpiresAt.Before(now) {
		return Challenge{}, fmt.Errorf("state expired or not found")
	}
	delete(s.states, stateID)

	ch, ok := s.challenges[state.ChallengeID]
	if !ok || ch.ExpiresAt.Before(now) {
		return Challenge{}, fmt.Errorf("challenge expired or not found")
	}
	delete(s.challenges, state.ChallengeID)
	return ch, nil
}

func (s *ChallengeStore) cleanupLocked(now time.Time) {
	for id, ch := range s.challenges {
		if ch.ExpiresAt.Before(now) {
			delete(s.challenges, id)
		}
	}
	for id, st := range s.states {
		if st.ExpiresAt.Before(now) {
			delete(s.states, id)
		}
	}
}
