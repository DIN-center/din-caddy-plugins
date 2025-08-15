package siwe

import (
	"github.com/DIN-center/din-caddy-plugins/lib/contracts/nftoptions"
	"github.com/umbracle/ethgo"
	"math/big"
	"math/rand"
	"sync"
	"time"
	"go.uber.org/zap"
)

type NFTSelector func([]*nftoptions.NFTMetadata) *nftoptions.NFTMetadata


type NFTManager struct {
	OptionsManager *nftoptions.Config
	Owner ethgo.Address
	nftsByProvider map[string][]*nftoptions.NFTMetadata
	mutex sync.RWMutex
	stopChan chan struct{}
	once sync.Once
	logger  *zap.Logger
}

func NewNFTManager(optionsManager *nftoptions.Config, owner string) *NFTManager {
	return &NFTManager{
		OptionsManager: optionsManager,
		Owner: ethgo.HexToAddress(owner),
		nftsByProvider: make(map[string][]*nftoptions.NFTMetadata),
		stopChan: make(chan struct{}),
	}
}

func (m *NFTManager) GetNFTForProvider(providerID *big.Int, selector NFTSelector) *nftoptions.NFTMetadata {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	providerKey := providerID.String()
	nfts := m.nftsByProvider[providerKey]
	if len(nfts) == 0 {
		return nil
	}

	if selector == nil {
		return nfts[rand.Intn(len(nfts))]
	}

	return selector(nfts)
}

func (m *NFTManager) Refresh() error {

	tokenIds, err := m.OptionsManager.TokenIDsByOwner(m.Owner)
	if err != nil {
		m.logger.Warn("error getting token ids by owner", zap.String("error", err.Error()))
		return err
	}

	now := uint64(time.Now().Unix())
	bufferTime := now + 60

	nftsByProvider := make(map[string][]*nftoptions.NFTMetadata) // Clear old data

	for _, tokenId := range tokenIds {
		metadata, err := m.OptionsManager.GetTokenMetadata(tokenId)
		if err != nil || metadata.Expiration <= bufferTime {
			m.logger.Warn("error getting token metadata", zap.String("error", err.Error()))
			continue
		}
		providerKey := metadata.ServiceId.String()
		nftsByProvider[providerKey] = append(m.nftsByProvider[providerKey], metadata)
	}

	if len(nftsByProvider) > 0 {
		m.logger.Debug("Loaded nfts", zap.Any("nftsByProvider", nftsByProvider))
		m.mutex.Lock()
		m.nftsByProvider = nftsByProvider
		m.mutex.Unlock()
	}

	return nil
}

func (m *NFTManager) Start(logger *zap.Logger) error {
	m.logger = logger
	var err error
	m.once.Do(func() {
		err = m.Refresh()
		go func() {
			time.Sleep(60 * time.Second)
			for {
				select {
				case <-m.stopChan:
					return
				default:
					m.Refresh()
					time.Sleep(60 * time.Second)
				}
			}
		}()
	})
	return err
}

func (m *NFTManager) Stop() {
	close(m.stopChan)
	m.once = sync.Once{}
}
