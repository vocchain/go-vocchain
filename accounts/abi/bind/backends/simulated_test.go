// Copyright 2019 The go-voc-core Authors
// This file is part of the go-voc-core library.
//
// The go-voc-core library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-voc-core library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-voc-core library. If not, see <http://www.gnu.org/licenses/>.

package backends_test

import (
	"context"
	"math/big"
	"testing"

	voc-core "github.com/voc-core/go-voc-core"
	"github.com/voc-core/go-voc-core/accounts/abi/bind"
	"github.com/voc-core/go-voc-core/accounts/abi/bind/backends"
	"github.com/voc-core/go-voc-core/common"
	"github.com/voc-core/go-voc-core/core"
	"github.com/voc-core/go-voc-core/core/types"
	"github.com/voc-core/go-voc-core/crypto"
)

func TestSimulatedBackend(t *testing.T) {
	var gasLimit uint64 = 8000029
	key, _ := crypto.GenerateKey() // nolint: gosec
	auth := bind.NewKeyedTransactor(key)
	genAlloc := make(core.GenesisAlloc)
	genAlloc[auth.From] = core.GenesisAccount{Balance: big.NewInt(9223372036854775807)}

	sim := backends.NewSimulatedBackend(genAlloc, gasLimit)
	defer sim.Close()

	// should return an error if the tx is not found
	txHash := common.HexToHash("2")
	_, isPending, err := sim.TransactionByHash(context.Background(), txHash)

	if isPending {
		t.Fatal("transaction should not be pending")
	}
	if err != voc-core.NotFound {
		t.Fatalf("err should be `voc-core.NotFound` but received %v", err)
	}

	// generate a transaction and confirm you can retrieve it
	code := `6060604052600a8060106000396000f360606040526008565b00`
	var gas uint64 = 3000000
	tx := types.NewContractCreation(0, big.NewInt(0), gas, big.NewInt(1), common.FromHex(code))
	tx, _ = types.SignTx(tx, types.HomesteadSigner{}, key)

	err = sim.SendTransaction(context.Background(), tx)
	if err != nil {
		t.Fatal("error sending transaction")
	}

	txHash = tx.Hash()
	_, isPending, err = sim.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatalf("error getting transaction with hash: %v", txHash.String())
	}
	if !isPending {
		t.Fatal("transaction should have pending status")
	}

	sim.Commit()
	_, isPending, err = sim.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatalf("error getting transaction with hash: %v", txHash.String())
	}
	if isPending {
		t.Fatal("transaction should not have pending status")
	}

}
