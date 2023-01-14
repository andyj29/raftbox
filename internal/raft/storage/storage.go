package storage

type Persister struct {
}

func (p *Persister) ReadState() []byte {
	return nil
}

func (p *Persister) SaveState(data []byte) {

}

func (p *Persister) ReadSnapshot() []byte {
	return nil
}
