package anchoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

// RPCClient is a minimal Ethereum JSON-RPC client (stdlib http only).
type RPCClient struct {
	URL  string
	HTTP *http.Client
	id   int
}

// NewRPCClient builds a client for a public RPC endpoint URL.
func NewRPCClient(url string) *RPCClient {
	return &RPCClient{
		URL:  url,
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message) }

func (c *RPCClient) call(method string, params []any) (json.RawMessage, error) {
	c.id++
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", method, err)
	}
	if envelope.Error != nil {
		return nil, envelope.Error
	}
	return envelope.Result, nil
}

func (c *RPCClient) callString(method string, params []any) (string, error) {
	raw, err := c.call(method, params)
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("%s: %w", method, err)
	}
	return s, nil
}

// ChainID returns the chain id of the connected network.
func (c *RPCClient) ChainID() (int64, error) {
	s, err := c.callString("eth_chainId", []any{})
	if err != nil {
		return 0, err
	}
	return parseHexInt64(s)
}

// TransactionCount returns the account nonce ("pending" block).
func (c *RPCClient) TransactionCount(address string) (int64, error) {
	s, err := c.callString("eth_getTransactionCount", []any{address, "pending"})
	if err != nil {
		return 0, err
	}
	return parseHexInt64(s)
}

// Balance returns the account balance in wei.
func (c *RPCClient) Balance(address string) (*big.Int, error) {
	s, err := c.callString("eth_getBalance", []any{address, "latest"})
	if err != nil {
		return nil, err
	}
	return parseHexBig(s)
}

// MaxPriorityFeePerGas returns the suggested priority fee (wei).
func (c *RPCClient) MaxPriorityFeePerGas() (*big.Int, error) {
	s, err := c.callString("eth_maxPriorityFeePerGas", []any{})
	if err != nil {
		return nil, err
	}
	return parseHexBig(s)
}

// GasPrice returns eth_gasPrice (wei).
func (c *RPCClient) GasPrice() (*big.Int, error) {
	s, err := c.callString("eth_gasPrice", []any{})
	if err != nil {
		return nil, err
	}
	return parseHexBig(s)
}

// EstimateGas estimates gas for a transaction object.
func (c *RPCClient) EstimateGas(tx map[string]any) (int64, error) {
	s, err := c.callString("eth_estimateGas", []any{tx})
	if err != nil {
		return 0, err
	}
	return parseHexInt64(s)
}

// SendRawTransaction broadcasts a signed transaction and returns its hash.
func (c *RPCClient) SendRawTransaction(raw []byte) (string, error) {
	return c.callString("eth_sendRawTransaction", []any{HexEncodeBytes(raw)})
}

type rpcTransaction struct {
	Hash  string `json:"hash"`
	Input string `json:"input"`
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
}

// GetTransaction fetches a transaction by hash.
func (c *RPCClient) GetTransaction(hash string) (*rpcTransaction, error) {
	raw, err := c.call("eth_getTransactionByHash", []any{hash})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("transaction %s not found", hash)
	}
	var tx rpcTransaction
	if err := json.Unmarshal(raw, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

type rpcReceipt struct {
	BlockNumber string `json:"blockNumber"`
	GasUsed     string `json:"gasUsed"`
	Status      string `json:"status"`
}

// GetTransactionReceipt fetches a mined transaction's receipt; returns
// (nil, nil) while the transaction is still pending.
func (c *RPCClient) GetTransactionReceipt(hash string) (*rpcReceipt, error) {
	raw, err := c.call("eth_getTransactionReceipt", []any{hash})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var r rpcReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// BlockTimestamp returns the timestamp (unix seconds) of a block.
func (c *RPCClient) BlockTimestamp(blockNumber int64) (int64, error) {
	raw, err := c.call("eth_getBlockByNumber", []any{fmt.Sprintf("0x%x", blockNumber), false})
	if err != nil {
		return 0, err
	}
	var block struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return 0, err
	}
	return parseHexInt64(block.Timestamp)
}

// LatestBaseFee returns the base fee of the latest block.
func (c *RPCClient) LatestBaseFee() (*big.Int, error) {
	raw, err := c.call("eth_getBlockByNumber", []any{"latest", false})
	if err != nil {
		return nil, err
	}
	var block struct {
		BaseFeePerGas string `json:"baseFeePerGas"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, err
	}
	return parseHexBig(block.BaseFeePerGas)
}

func parseHexInt64(s string) (int64, error) {
	n, err := parseHexBig(s)
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

func parseHexBig(s string) (*big.Int, error) {
	// JSON-RPC quantities are 0x-prefixed with no leading zeros, so a value
	// like "0x0" has an odd nibble count — left-pad before decoding.
	t := s
	if len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		t = t[2:]
	}
	if len(t)%2 != 0 {
		t = "0" + t
	}
	b, err := HexDecode(t)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}

// DefaultRPCURL returns the public RPC endpoint for a chain id.
func DefaultRPCURL(chainID int64) (string, bool) {
	switch chainID {
	case 8453:
		return "https://mainnet.base.org", true
	case 84532:
		return "https://sepolia.base.org", true
	}
	return "", false
}

// ChainName labels a chain id.
func ChainName(chainID int64) string {
	switch chainID {
	case 8453:
		return "base-mainnet"
	case 84532:
		return "base-sepolia"
	}
	return fmt.Sprintf("chain-%d", chainID)
}
