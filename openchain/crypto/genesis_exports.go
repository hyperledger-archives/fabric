package crypto

type Genesis interface {
	MakeGenesis() error
}