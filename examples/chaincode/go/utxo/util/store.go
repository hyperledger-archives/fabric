package util

type Store interface {
	GetState(Key) (*TX_TXOUT, bool, error)
	PutState(Key, *TX_TXOUT) error
	DelState(Key) error
	GetTran(string) ([]byte, bool, error)
	PutTran(string, []byte) error
}

type InMemoryStore struct {
	Map     map[Key]*TX_TXOUT
	TranMap map[string][]byte
}

func MakeInMemoryStore() Store {
	ims := &InMemoryStore{}
	ims.Map = make(map[Key]*TX_TXOUT)
	ims.TranMap = make(map[string][]byte)
	return ims
}

func (ims *InMemoryStore) GetState(key Key) (*TX_TXOUT, bool, error) {
	value, ok := ims.Map[key]
	return value, ok, nil
}

func (ims *InMemoryStore) DelState(key Key) error {
	delete(ims.Map, key)
	return nil
}

func (ims *InMemoryStore) PutState(key Key, value *TX_TXOUT) error {
	ims.Map[key] = value
	return nil
}

func (ims *InMemoryStore) GetTran(key string) ([]byte, bool, error) {
	value, ok := ims.TranMap[key]
	return value, ok, nil
}

func (ims *InMemoryStore) PutTran(key string, value []byte) error {
	ims.TranMap[key] = value
	return nil
}
