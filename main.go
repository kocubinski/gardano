package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"
	"time"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/blockfetch"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	"github.com/blinklabs-io/gouroboros/protocol/common"
	"github.com/blinklabs-io/gouroboros/protocol/localstatequery"
	"github.com/gcash/bchutil/bech32"
	"github.com/kocubinski/gardano/address"
	"github.com/kocubinski/gardano/cbor"
	"github.com/kocubinski/gardano/tx"
)

const (
	testnetMagic = 42
	mainnetMagic = 764824073
)

// 1 input 1 output tx fee:
// 1 input 2 outputs tx fee: 169329

type cliFlags struct {
	flagset *flag.FlagSet

	networkMagic uint32

	// key pair
	seed string

	// client
	clientAddress string
	clientSocket  string

	// Tx submission
	receiverAddress string
	sendAmount      uint64
	memo            string
	fee             uint64

	// chain sync
	filterAddresses string
	startHash       string
	startSlot       uint64
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gardano <command>")
		os.Exit(1)
	}
	command := os.Args[1]

	f := &cliFlags{
		flagset: flag.NewFlagSet(command, flag.ExitOnError),
	}
	parseFlags := func() {
		if err := f.flagset.Parse(os.Args[2:]); err != nil {
			fmt.Println("failed to parse flags:", err)
			os.Exit(1)
		}
	}
	var err error
	switch command {
	case "send-tx":
		f.flagset.StringVar(&f.receiverAddress, "receiver-address", "", "Address to send to")
		f.flagset.Uint64Var(&f.sendAmount, "amount", 0, "Amount to send")
		f.flagset.StringVar(&f.clientAddress, "address", "", "TCP address for n2c communication")
		f.flagset.StringVar(&f.clientSocket, "socket", "", "unix socket address for n2c communication")
		f.flagset.StringVar(&f.memo, "memo", "", "optional tx memo")
		f.flagset.Uint64Var(&f.fee, "fee", 0, "if unset fees are dynamically calculated")
		var networkMagic uint
		f.flagset.UintVar(&networkMagic, "magic", testnetMagic, "network magic")
		parseFlags()
		f.networkMagic = uint32(networkMagic)
		err = sendTx(f)
	case "chain-sync":
		f.flagset.StringVar(&f.filterAddresses, "filter-addresses", "", "Filter addresses")
		f.flagset.StringVar(&f.startHash, "start-hash", "", "Start hash")
		f.flagset.Uint64Var(&f.startSlot, "start-slot", 0, "Start slot")
		parseFlags()
		err = runNode(f)
	case "key-pair":
		f.flagset.StringVar(&f.seed, "seed", "", "random seed for key pair")
		var networkMagic uint
		f.flagset.UintVar(&networkMagic, "magic", testnetMagic, "network magic")
		parseFlags()
		f.networkMagic = uint32(networkMagic)
		err = makeKeyPair(f)
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
)

func makeKeyPair(f *cliFlags) error {
	seedBz, err := hex.DecodeString(f.seed)
	if err != nil {
		return err
	}
	fmt.Printf("seed: %+v\n", seedBz)
	var buf bytes.Buffer
	buf.Write(seedBz)
	pub, priv, err := ed25519.GenerateKey(&buf)
	if err != nil {
		return err
	}
	var addr address.Address
	switch f.networkMagic {
	case 764824073:
		addr, err = address.PaymentOnlyMainnetAddressFromPubkey(pub)
	case 42:
		addr, err = address.PaymentOnlyTestnetAddressFromPubkey(pub)
	default:
		return fmt.Errorf("unknown network magic: %d", f.networkMagic)
	}
	if err != nil {
		return err
	}

	pub5Bit, err := bech32.ConvertBits(pub, 8, 5, true)
	if err != nil {
		return err
	}
	pubBech32, err := bech32.Encode("addr_vk", pub5Bit)
	if err != nil {
		return err
	}

	priv5Bit, err := bech32.ConvertBits(priv.Seed(), 8, 5, true)
	if err != nil {
		return err
	}
	privBech32, err := bech32.Encode("addr_sk", priv5Bit)
	if err != nil {
		return err
	}

	fmt.Printf("addr: %s\n", addr.String())
	fmt.Printf(" pub: %s\n", pubBech32)
	fmt.Printf("priv: %s\n", privBech32)
	return nil
}

func getPrivateKey() (ed25519.PrivateKey, error) {
	if signKeyBech32 := os.Getenv("CARDANO_SIGNING_KEY_BECH32"); signKeyBech32 != "" {
		return address.PrivateKeyFromBech32(signKeyBech32)
	}
	if signKeyHex := os.Getenv("CARDANO_SIGNING_KEY_CBOR"); signKeyHex != "" {
		cborBz, err := hex.DecodeString(signKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex: %w", err)
		}
		var keyBz []byte
		_, err = cbor.Decode(cborBz, &keyBz)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cbor: %w", err)
		}
		return ed25519.NewKeyFromSeed(keyBz), nil
	}
	return nil, fmt.Errorf("either CARDANO_SIGNING_KEY_BECH32 or CARDANO_SIGNING_KEY_CBOR must be set")
}

func sendTx(f *cliFlags) error {
	if f.clientAddress == "" && f.clientSocket == "" {
		return fmt.Errorf("client address/socket is not set")
	}
	if f.sendAmount == 0 {
		return fmt.Errorf("send amount is not set")
	}
	if f.receiverAddress == "" {
		return fmt.Errorf("receiver address is not set")
	}

	priv, err := getPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to create private key: %w", err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	var sourceAddr address.Address
	if f.networkMagic == 42 {
		sourceAddr, err = address.PaymentOnlyTestnetAddressFromPubkey(pub)
	} else {
		sourceAddr, err = address.PaymentOnlyMainnetAddressFromPubkey(pub)
	}
	if err != nil {
		return err
	}
	addr, err := ledger.NewAddress(sourceAddr.String())
	if err != nil {
		return fmt.Errorf("failed to create address: %w", err)
	}

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
	network, ok := ouroboros.NetworkByNetworkMagic(f.networkMagic)
	if !ok {
		return fmt.Errorf("unknown network magic: %d", f.networkMagic)
	}
	var client net.Conn
	if f.clientSocket != "" {
		client, err = net.Dial("unix", f.clientSocket)
	} else {
		client, err = net.Dial("tcp", f.clientAddress)
	}
	if err != nil {
		return fmt.Errorf("failed to create client connection: %w", err)
	}
	o, err := ouroboros.NewConnection(
		ouroboros.WithConnection(client),
		ouroboros.WithErrorChan(errorChan),
		ouroboros.WithLogger(log),
		ouroboros.WithNetwork(network),
		ouroboros.WithLocalStateQueryConfig(localstatequery.NewConfig()),
		ouroboros.WithKeepAlive(true),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to network: %w", err)
	}
	pparams, err := o.LocalStateQuery().Client.GetCurrentProtocolParams()
	if err != nil {
		return fmt.Errorf("failed to load protocol parameters: %w", err)
	}
	txBuilder := tx.NewTxBuilder(pparams.Utxorpc(), []ed25519.PrivateKey{priv})

	utxoRes, err := o.LocalStateQuery().Client.GetUTxOByAddress([]ledger.Address{addr})
	if err != nil {
		return fmt.Errorf("failed to get utxo: %w", err)
	}

	estimatedFee := uint64(167217)
	if f.fee > 0 {
		estimatedFee = f.fee
	}
	minRequired := f.sendAmount + estimatedFee // estimate fee
	txIns, err := maxNumberUTxOs(utxoRes, uint64(minRequired))
	if err != nil {
		return err
	}
	txBuilder.AddInputs(txIns...)

	toAddr, err := address.NewAddressFromBech32(f.receiverAddress)
	if err != nil {
		return fmt.Errorf("failed to create address: %w", err)
	}
	txBuilder.AddOutputs(tx.NewTxOutput(toAddr, f.sendAmount))

	tip, err := o.ChainSync().Client.GetCurrentTip()
	if err != nil {
		return fmt.Errorf("failed to get current tip for TTL: %w", err)
	}
	txBuilder.SetTTL(uint32(tip.Point.Slot + 300))

	if err := txBuilder.AddChangeIfNeeded(sourceAddr); err != nil {
		return fmt.Errorf("failed to add change: %w", err)
	}

	// set memo if present
	if f.memo != "" {
		if err := txBuilder.SetMemo(f.memo); err != nil {
			return fmt.Errorf("failed to set memo: %w", err)
		}
	}
	txFinal, err := txBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build transaction: %w", err)
	}
	txBz, err := txFinal.Bytes()
	if err != nil {
		return fmt.Errorf("failed to get transaction bytes: %w", err)
	}
	jsonBz, err := json.MarshalIndent(txFinal, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to json marshal transaction: %w", err)
	}
	fmt.Printf("txFinal:\n%s\n", jsonBz)

	era, err := o.LocalStateQuery().Client.GetCurrentEra()
	if err != nil {
		return fmt.Errorf("failed to get current era: %w", err)
	}
	err = o.LocalTxSubmission().Client.SubmitTx(uint16(era), txBz)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	return nil
}

func runNode(f *cliFlags) error {
	if f.filterAddresses != "" {
		filterAddresses = strings.Split(f.filterAddresses, ",")
	}
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
	network, ok := ouroboros.NetworkByNetworkMagic(f.networkMagic)
	if !ok {
		return fmt.Errorf("unknown network magic: %d", f.networkMagic)
	}
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

	var point common.Point
	if f.startHash != "" && f.startSlot != 0 {
		h, err := hex.DecodeString(f.startHash)
		if err != nil {
			return fmt.Errorf("failed to decode start hash: %w", err)
		}
		point = common.Point{
			Slot: f.startSlot,
			Hash: h,
		}
	} else {
		tip, err := o.ChainSync().Client.GetCurrentTip()
		if err != nil {
			return fmt.Errorf("failed to get current tip: %w", err)
		}
		point = tip.Point
		fmt.Printf("queried tip: slot = %d, hash = %x\n", point.Slot, point.Hash)
	}

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
		// chainsync.WithRollForwardRawFunc(chainSyncRollForwardRawHandler),
	)
}

func buildBlockFetchConfig() blockfetch.Config {
	return blockfetch.NewConfig(
		blockfetch.WithBlockFunc(blockFetchBlockHandler),
	)
}

var filterAddresses []string

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
		fmt.Printf("block header, fetching block (%d, %x)\n", blockSlot, blockHash)
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
		if len(filterAddresses) > 0 {
			for _, blockTx := range block.Transactions() {
				for _, txOut := range blockTx.Outputs() {
					for _, addr := range filterAddresses {
						if addr == txOut.Address().String() {
							fmt.Printf("filter tx:\n  tx-hash: %s\n  address: %s\n  amount: %d\n",
								blockTx.Hash(),
								txOut.Address(),
								txOut.Amount(),
							)
							memo, err := tx.DecodeMemo(blockTx)
							if err != nil {
								fmt.Printf("ERROR: failed to decode memo: %s\n", err)
							} else if memo != "" {
								fmt.Printf("  memo: %s\n", memo)
							}
						}
					}
				}
			}
		}
	}

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

func chainSyncRollForwardRawHandler(
	ctx chainsync.CallbackContext,
	blockType uint,
	rawBlockData []byte,
	tip chainsync.Tip,
) error {
	fmt.Printf("roll forward raw: tip = (%d, %x) bytes = %d\n", tip.Point.Slot, tip.Point.Hash, len(rawBlockData))
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

func createClientConnection(address string, useTls bool) (net.Conn, error) {
	if useTls {
		return tls.Dial("tcp", address, nil)
	} else {
		return net.Dial("tcp", address)
	}
}

func maxNumberUTxOs(utxoRes *localstatequery.UTxOByAddressResult, targetAmount uint64) ([]tx.TxInput, error) {
	var txIns []tx.TxInput
	for txId, txOut := range utxoRes.Results {
		txIn := tx.NewTxInput(txId.Hash.String(), uint16(txId.Idx), txOut.Amount())
		txIns = append(txIns, txIn)
		fmt.Printf("txId: %s, txOut: %d\n", txId.Hash.String(), txOut.Amount())
	}
	slices.SortFunc(txIns, func(a, b tx.TxInput) int {
		switch {
		case a.Amount < b.Amount:
			return -1
		case a.Amount > b.Amount:
			return 1
		default:
			return 0
		}
	})

	var res []tx.TxInput
	for _, txIn := range txIns {
		res = append(res, txIn)
		if txIn.Amount > targetAmount {
			targetAmount = 0
			break
		}
		targetAmount -= txIn.Amount
	}

	if targetAmount > 0 {
		return nil, fmt.Errorf("insufficient funds; short by %d", targetAmount)
	}

	return res, nil
}
