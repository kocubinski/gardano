package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/blockfetch"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	"github.com/blinklabs-io/gouroboros/protocol/common"
	"github.com/blinklabs-io/gouroboros/protocol/localstatequery"
	"github.com/blinklabs-io/gouroboros/protocol/localtxsubmission"
	"github.com/blinklabs-io/gouroboros/protocol/txsubmission"
	"github.com/kocubinski/go-cardano/address"
	"github.com/kocubinski/go-cardano/protocol"
	"github.com/kocubinski/go-cardano/tx"
)

type cliFlags struct {
	flagset *flag.FlagSet

	// client
	clientAddress string

	// Tx submission
	receiverAddress string
	pparams         string
	sendAmount      uint
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go-cardano <command>")
		os.Exit(1)
	}
	command := os.Args[1]

	fmt.Println("command:", command)
	f := &cliFlags{
		flagset: flag.NewFlagSet(command, flag.ExitOnError),
	}
	var err error
	switch command {
	case "send-tx":
		f.flagset.StringVar(&f.receiverAddress, "receiver-address", "", "Address to send to")
		f.flagset.StringVar(&f.pparams, "protocol-parameters-file", "", "Path to protocol parameters file")
		f.flagset.UintVar(&f.sendAmount, "amount", 0, "Amount to send")
		f.flagset.StringVar(&f.clientAddress, "address", "", "socket address for n2c communication")
		if err := f.flagset.Parse(os.Args[2:]); err != nil {
			fmt.Println("failed to parse flags:", err)
			os.Exit(1)
		}
		err = sendTx(f)
	case "run-node":
		err = runNode()
	case "debug-tx":
		f.flagset.StringVar(&f.clientAddress, "address", "", "socket address for n2c communication")
		if err := f.flagset.Parse(os.Args[2:]); err != nil {
			fmt.Println("failed to parse flags:", err)
			os.Exit(1)
		}
		err = debugTx(f)
	default:
		fmt.Println("unknown command")
		os.Exit(1)
	}
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}

var (
	ouroborosConnection *ouroboros.Connection
	shutdownWait        sync.WaitGroup
	startHash           = "5baf72ec3430f0f65b5b7356d2b15720e451ac6593652bcbea4dd60a1ab99ebd"
	startSlot           = uint64(148785568)
)

func debugTx(f *cliFlags) error {
	signKeyBech32 := os.Getenv("CARDANO_SIGNING_KEY_BECH32")
	if signKeyBech32 == "" {
		return fmt.Errorf("CARDANO_SIGNING_KEY_BECH32 is not set")
	}

	return nil
}

func sendTx(f *cliFlags) error {
	signKeyBech32 := os.Getenv("CARDANO_SIGNING_KEY_BECH32")
	if signKeyBech32 == "" {
		return fmt.Errorf("CARDANO_SIGNING_KEY_BECH32 is not set")
	}
	if f.clientAddress == "" {
		return fmt.Errorf("client address is not set")
	}
	if f.pparams == "" {
		return fmt.Errorf("protocol parameters file is not set")
	}
	if f.sendAmount == 0 {
		return fmt.Errorf("send amount is not set")
	}
	if f.receiverAddress == "" {
		return fmt.Errorf("receiver address is not set")
	}

	priv, err := address.PrivateKeyFromBech32(signKeyBech32)
	if err != nil {
		return fmt.Errorf("failed to create private key: %w", err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	sourceAddr, err := address.NewMainnetPaymentOnlyFromPubkey(pub)
	if err != nil {
		return err
	}
	addr, err := ledger.NewAddress(sourceAddr.String())
	if err != nil {
		return fmt.Errorf("failed to create address: %w", err)
	}
	pparams, err := protocol.LoadProtocol(f.pparams)
	if err != nil {
		return fmt.Errorf("failed to load protocol parameters: %w", err)
	}
	txBuilder := tx.NewTxBuilder(pparams, []ed25519.PrivateKey{priv})

	errorChan := make(chan error)
	go func() {
		for {
			err := <-errorChan
			fmt.Printf("ERROR(async): %s\n", err)
			// os.Exit(1)
		}
	}()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	network := ouroboros.NetworkMainnet
	client, err := createClientConnection(f.clientAddress, false)
	if err != nil {
		return fmt.Errorf("failed to create client connection: %w", err)
	}
	o, err := ouroboros.NewConnection(
		ouroboros.WithConnection(client),
		ouroboros.WithErrorChan(errorChan),
		ouroboros.WithLogger(log),
		ouroboros.WithNetwork(network),
		ouroboros.WithLocalStateQueryConfig(localstatequery.NewConfig()),
		ouroboros.WithLocalTxSubmissionConfig(
			localtxsubmission.NewConfig(
				localtxsubmission.WithSubmitTxFunc(localSubmitTxCallbackHandler),
			),
		),
		ouroboros.WithKeepAlive(true),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to network: %w", err)
	}
	utxoRes, err := o.LocalStateQuery().Client.GetUTxOByAddress([]ledger.Address{addr})
	if err != nil {
		return fmt.Errorf("failed to get utxo: %w", err)
	}

	var txIn *tx.TxInput
	minRequired := f.sendAmount + 167217 // estimate fee
	for txId, txOut := range utxoRes.Results {
		utxoAmount := uint(txOut.Amount())
		fmt.Printf("tx-hash: %s, tx-idx: %d, address: %s, amount: %d\n",
			txId.Hash.String(),
			txId.Idx,
			txOut.Address(),
			utxoAmount,
		)
		if utxoAmount >= minRequired {
			txIn = tx.NewTxInput(txId.Hash.String(), uint16(txId.Idx), uint(txOut.Amount()))
			break
		}
	}
	if txIn == nil {
		return fmt.Errorf("no matching utxo found")
	}
	txBuilder.AddInputs(txIn)
	txBuilder.AddOutputs(tx.NewTxOutput(address.MustFromBech32(f.receiverAddress), f.sendAmount))
	// tip, err := o.ChainSync().Client.GetCurrentTip()
	// if err != nil {
	// 	return fmt.Errorf("failed to get current tip: %w", err)
	// }
	era, err := o.LocalStateQuery().Client.GetCurrentEra()
	if err != nil {
		return fmt.Errorf("failed to get current era: %w", err)
	}
	// txBuilder.SetTTL(uint32(tip.Point.Slot + 300))
	if err := txBuilder.AddChangeIfNeeded(sourceAddr); err != nil {
		return fmt.Errorf("failed to add change: %w", err)
	}
	txFinal, err := txBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build transaction: %w", err)
	}
	fmt.Println("txFinal:", txFinal)
	txBz, err := txFinal.Bytes()
	if err != nil {
		return fmt.Errorf("failed to get transaction bytes: %w", err)
	}
	jsonBz, err := json.MarshalIndent(txFinal, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to json marshal transaction: %w", err)
	}
	fmt.Printf("txFinal:\n%s\n", jsonBz)
	shutdownWait.Add(1)
	err = o.LocalTxSubmission().Client.SubmitTx(uint16(era), txBz)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}
	shutdownWait.Wait()

	return nil
}

func runNode() error {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	// Create error channel
	networkError := make(chan error)
	// start error handler
	go func() {
		for {
			err := <-networkError
			log.Error("network error", "error", err)
			time.Sleep(1 * time.Second)
		}
	}()
	network := ouroboros.NetworkMainnet
	peer := network.BootstrapPeers[0]
	client, err := createClientConnection(fmt.Sprintf("%s:%d", peer.Address, peer.Port), false)
	if err != nil {
		return fmt.Errorf("failed to create client connection: %w", err)
	}
	o, err := ouroboros.NewConnection(
		ouroboros.WithPeerSharing(true),
		ouroboros.WithConnection(client),
		ouroboros.WithNetwork(network),
		ouroboros.WithLogger(log),
		ouroboros.WithErrorChan(networkError),
		ouroboros.WithNodeToNode(true),
		ouroboros.WithKeepAlive(true),
		ouroboros.WithChainSyncConfig(buildChainSyncConfig()),
		ouroboros.WithBlockFetchConfig(buildBlockFetchConfig()),
		// ouroboros.WithLocalStateQueryConfig(localstatequery.NewConfig()),
		// ouroboros.WithTxSubmissionConfig(buildTxSubmissionConfig()),
	)
	ouroborosConnection = o
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	tip, err := o.ChainSync().Client.GetCurrentTip()
	if err != nil {
		return fmt.Errorf("failed to get current tip: %w", err)
	}
	fmt.Printf("tip: slot = %d, hash = %x\n", tip.Point.Slot, tip.Point.Hash)

	h, err := hex.DecodeString(startHash)
	if err != nil {
		return fmt.Errorf("failed to decode start hash: %w", err)
	}
	point := common.Point{Slot: startSlot, Hash: h}
	if err := o.ChainSync().Client.Sync([]common.Point{point}); err != nil {
		fmt.Printf("ERROR: failed to start chain-sync: %s\n", err)
		os.Exit(1)
	}

	// o.TxSubmission().Client.Init()

	select {}
}

func buildChainSyncConfig() chainsync.Config {
	return chainsync.NewConfig(
		chainsync.WithRollBackwardFunc(chainSyncRollBackwardHandler),
		chainsync.WithRollForwardFunc(chainSyncRollForwardHandler),
	)
}

func buildBlockFetchConfig() blockfetch.Config {
	return blockfetch.NewConfig(
		blockfetch.WithBlockFunc(blockFetchBlockHandler),
	)
}

func buildTxSubmissionConfig() txsubmission.Config {
	return txsubmission.NewConfig(
		txsubmission.WithRequestTxsFunc(txSubmissionRequestHandler),
		txsubmission.WithRequestTxIdsFunc(txSubmissionIdsRequestHandler),
	)
}

func chainSyncRollForwardHandler(
	ctx chainsync.CallbackContext,
	blockType uint,
	blockData interface{},
	tip chainsync.Tip,
) error {
	var block ledger.Block
	switch v := blockData.(type) {
	case ledger.Block:
		block = v
	case ledger.BlockHeader:
		blockSlot := v.SlotNumber()
		blockHash, _ := hex.DecodeString(v.Hash())
		// fmt.Printf("block header, fetching block (%d, %x)\n", blockSlot, blockHash)
		var err error
		block, err = ouroborosConnection.BlockFetch().Client.GetBlock(common.NewPoint(blockSlot, blockHash))
		if err != nil {
			return err
		}
	}
	// Display block info
	switch blockType {
	case ledger.BlockTypeByronEbb:
		byronEbbBlock := block.(*ledger.ByronEpochBoundaryBlock)
		fmt.Printf(
			"era = Byron (EBB), epoch = %d, slot = %d, id = %s\n",
			byronEbbBlock.BlockHeader.ConsensusData.Epoch,
			byronEbbBlock.SlotNumber(),
			byronEbbBlock.Hash(),
		)
	case ledger.BlockTypeByronMain:
		byronBlock := block.(*ledger.ByronMainBlock)
		fmt.Printf(
			"era = Byron, epoch = %d, slot = %d, id = %s\n",
			byronBlock.BlockHeader.ConsensusData.SlotId.Epoch,
			byronBlock.SlotNumber(),
			byronBlock.Hash(),
		)
	default:
		if block == nil {
			return fmt.Errorf("block is nil")
		}
		fmt.Printf(
			"era = %s, slot = %d, block_no = %d, hash = %s, txs=%d\n",
			block.Era().Name,
			block.SlotNumber(),
			block.BlockNumber(),
			block.Hash(),
			len(block.Transactions()),
		)
	}
	// err := ouroborosConnection.LocalStateQuery().Client.AcquireImmutableTip()
	// if err != nil {
	// 	return fmt.Errorf("failed to acquire immutable tip: %w", err)
	// }
	// immutableBlockNo, err := ouroborosConnection.LocalStateQuery().Client.GetChainBlockNo()
	// if err != nil {
	// 	return fmt.Errorf("failed to get chain block number: %w", err)
	// }
	// immutablePoint, err := ouroborosConnection.LocalStateQuery().Client.GetChainPoint()
	// if err != nil {
	// 	return fmt.Errorf("failed to get chain point: %w", err)
	// }
	// fmt.Printf(
	// 	"immutable tip: blockNo = %d, slot = %d, hash = %x\n",
	// 	immutableBlockNo, immutablePoint.Slot, immutablePoint.Hash,
	// )
	// err = ouroborosConnection.LocalStateQuery().Client.Release()
	// if err != nil {
	// 	return fmt.Errorf("failed to release immutable tip: %w", err)
	// }

	return nil
}

func blockFetchBlockHandler(
	ctx blockfetch.CallbackContext,
	blockType uint,
	blockData ledger.Block,
) error {
	switch block := blockData.(type) {
	case *ledger.ByronEpochBoundaryBlock:
		fmt.Printf("era = Byron (EBB), epoch = %d, slot = %d, id = %s\n", block.BlockHeader.ConsensusData.Epoch, block.SlotNumber(), block.Hash())
	case *ledger.ByronMainBlock:
		fmt.Printf("era = Byron, epoch = %d, slot = %d, id = %s\n", block.BlockHeader.ConsensusData.SlotId.Epoch, block.SlotNumber(), block.Hash())
	case ledger.Block:
		fmt.Printf("era = %s, slot = %d, block_no = %d, id = %s\n", block.Era().Name, block.SlotNumber(), block.BlockNumber(), block.Hash())
	}
	return nil
}

func chainSyncRollBackwardHandler(
	ctx chainsync.CallbackContext,
	point common.Point,
	tip chainsync.Tip,
) error {
	fmt.Printf("roll backward: point = (%d, %x), tip = (%d, %x)\n",
		point.Slot, point.Hash,
		tip.Point.Slot, tip.Point.Hash,
	)
	return nil
}

func txSubmissionRequestHandler(
	ctx txsubmission.CallbackContext, txs []txsubmission.TxId,
) ([]txsubmission.TxBody, error) {
	fmt.Printf("Request TxIds count=%d\n", len(txs))
	for _, tx := range txs {
		fmt.Printf("TxId: %x\n", tx)
	}
	return []txsubmission.TxBody{}, nil
}

func txSubmissionIdsRequestHandler(
	ctx txsubmission.CallbackContext, blocking bool, count uint16, size uint16,
) ([]txsubmission.TxIdAndSize, error) {
	fmt.Printf("txRequest: blocking=%t count=%d size=%d\n", blocking, count, size)
	return []txsubmission.TxIdAndSize{}, nil
}

func localSubmitTxCallbackHandler(
	ctx localtxsubmission.CallbackContext,
	msg localtxsubmission.MsgSubmitTxTransaction,
) error {
	fmt.Printf("localSubmitTxCallbackHandler: era=%x\n", msg.EraId)
	time.Sleep(5 * time.Second)
	shutdownWait.Done()
	return nil
}

func createClientConnection(address string, useTls bool) (net.Conn, error) {
	if useTls {
		return tls.Dial("tcp", address, nil)
	} else {
		return net.Dial("tcp", address)
	}
}
