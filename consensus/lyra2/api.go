package lyra2

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var errLyra2Stopped = errors.New("lyra2 stopped")

// API exposes lyra2 related methods for the RPC interface.
type API struct {
	lyra2 *Lyra2
}

// GetWork returns a work package for external miner.
//
// The work package consists of 3 strings:
//
//	result[0], 32 bytes hex encoded current block header pow-hash
//	result[1], hex encoded header
//	result[2], 32 bytes hex encoded boundary condition ("target"), 2^256/difficulty
//	result[3], hex encoded block number
func (api *API) GetWork() ([4]string, error) {
	if api.lyra2.remote == nil {
		return [4]string{}, errors.New("not supported")
	}

	var (
		workCh = make(chan [4]string, 1)
		errc   = make(chan error, 1)
	)
	select {
	case api.lyra2.remote.fetchWorkCh <- &sealWork{errc: errc, res: workCh}:
	case <-api.lyra2.remote.exitCh:
		return [4]string{}, errLyra2Stopped
	}
	select {
	case work := <-workCh:
		return work, nil
	case err := <-errc:
		return [4]string{}, err
	}
}

// SubmitWork can be used by external miner to submit their POW solution.
// It returns an indication if the work was accepted.
// Note either an invalid solution, a stale work a non-existent work will return false.
func (api *API) SubmitWork(nonce types.BlockNonce, hash, digest common.Hash) bool {
	if api.lyra2.remote == nil {
		return false
	}

	var errc = make(chan error, 1)
	select {
	case api.lyra2.remote.submitWorkCh <- &mineResult{
		nonce:     nonce,
		mixDigest: digest,
		hash:      hash,
		errc:      errc,
	}:
	case <-api.lyra2.remote.exitCh:
		return false
	}
	err := <-errc
	return err == nil
}

// SubmitHashRate can be used for remote miners to submit their hash rate.
// This enables the node to report the combined hash rate of all miners
// which submit work through this node.
//
// It accepts the miner hash rate and an identifier which must be unique
// between nodes.
func (api *API) SubmitHashRate(rate hexutil.Uint64, id common.Hash) bool {
	if api.lyra2.remote == nil {
		return false
	}

	var done = make(chan struct{}, 1)
	select {
	case api.lyra2.remote.submitRateCh <- &hashrate{done: done, rate: uint64(rate), id: id}:
	case <-api.lyra2.remote.exitCh:
		return false
	}

	// Block until hash rate submitted successfully.
	<-done
	return true
}

// GetHashrate returns the current hashrate for local CPU miner and remote miner.
func (api *API) GetHashrate() uint64 {
	return uint64(api.lyra2.Hashrate())
}
