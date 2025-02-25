package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/blockfetch"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	"github.com/blinklabs-io/gouroboros/protocol/common"
	"github.com/blinklabs-io/gouroboros/protocol/txsubmission"
	"github.com/gcash/bchutil/bech32"
)

type cliFlags struct {
	flagset *flag.FlagSet

	// Chain sync
	useTls       bool
	n2n          bool
	network      string
	networkMagic int

	// Tx submission
	sendAddress string
	txIn        string
	pparams     string
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
		f.flagset.StringVar(&f.sendAddress, "send-address", "", "Address to send to")
		f.flagset.StringVar(&f.pparams, "protocol-parameters-file", "", "Path to protocol parameters file")
		err = sendTx(f)
	case "run-node":
		err = runNode()
	case "key-sandbox":
		err = keySandbox(f)
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
	startHash           = "5baf72ec3430f0f65b5b7356d2b15720e451ac6593652bcbea4dd60a1ab99ebd"
	startSlot           = uint64(148785568)
)

func keySandbox(f *cliFlags) error {
	signKeyCbor := os.Getenv("CARDANO_SIGNING_KEY_CBOR")
	if signKeyCbor == "" {
		return fmt.Errorf("CARDANO_SIGNING_KEY_CBOR is not set")
	}
	cborBz, err := hex.DecodeString(signKeyCbor)
	if err != nil {
		return fmt.Errorf("failed to decode cbor: %w", err)
	}
	var bz []byte
	if _, err := cbor.Decode(cborBz, &bz); err != nil {
		return fmt.Errorf("failed to decode cbor: %w", err)
	}
	fmt.Printf("decoded: %d\n", len(bz))
	priv := ed25519.NewKeyFromSeed(bz)
	pub := priv.Public().(ed25519.PublicKey)
	privKeyCbor, err := cbor.Encode(priv.Seed())
	if err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}
	pubKeyCbor, err := cbor.Encode(pub)
	if err != nil {
		return fmt.Errorf("failed to encode public key: %w", err)
	}
	fmt.Printf("private key: %x\n", privKeyCbor)
	fmt.Printf(" public key: %x\n", pubKeyCbor)
	return nil
}

func sendTx(f *cliFlags) error {
	signKeyBech32 := os.Getenv("CARDANO_SIGNING_KEY_BECH32")
	if signKeyBech32 == "" {
		return fmt.Errorf("CARDANO_SIGNING_KEY_BECH32 is not set")
	}
	hrp, data, err := bech32.Decode(signKeyBech32)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}
	if hrp != "addr_sk" {
		return fmt.Errorf("invalid private key")
	}
	// signing key is the private key portion of an ed25519 keypair
	skey, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return fmt.Errorf("failed to convert bits: %w", err)
	}
	priv := ed25519.NewKeyFromSeed(skey)
	pub := priv.Public().(ed25519.PublicKey)
	data, err = bech32.ConvertBits(pub, 8, 5, true)
	if err != nil {
		return fmt.Errorf("failed to convert bits: %w", err)
	}
	pubBech32, err := bech32.Encode("addr_vk", data)
	if err != nil {
		return fmt.Errorf("failed to encode public key: %w", err)
	}
	fmt.Println("private key:", signKeyBech32)
	fmt.Println("public key:", pubBech32)

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

func createClientConnection(address string, useTls bool) (net.Conn, error) {
	if useTls {
		return tls.Dial("tcp", address, nil)
	} else {
		return net.Dial("tcp", address)
	}
}
