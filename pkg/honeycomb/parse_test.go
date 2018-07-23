package honeycomb

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParserReal(t *testing.T) {
	result, err := Parse("Parser1", []byte("Block{\n  Header{\n    ChainID:        test-chain-HMwr8o\n    Height:         23449\n    Time:           2018-07-22 23:44:32.6592659 +0000 UTC\n    NumTxs:         0\n    TotalTxs:       0\n    LastBlockID:    715A95922538778F136A65D86D974A3B273EA66E:1:2E904B3E22B2\n    LastCommit:     215A957565E17E4AEE20CB3E1667FBF15697A140\n    Data:           \n    Validators:     967B6BC72DD16F1695C763BFD27C8B64A81EA519\n    App:            D7F0E5DABF8CFA67182FFAC50526DF4F010841BA\n    Consensus:       D6B74BB35BDFFD8392340F2A379173548AE188FE\n    Results:        \n    Evidence:       \n  }#CF6F909CF3EE0316723FE043F0E50F6D0F4C166C\n  Data{\n    \n  }#\n  EvidenceData{\n    \n  }#\n  Commit{\n    BlockID:    715A95922538778F136A65D86D974A3B273EA66E:1:2E904B3E22B2\n    Precommits: Vote{0:EA0DD2EB887E 23448/00/2(Precommit) 715A95922538 /7301270C6CD1.../ @ 2018-07-22T23:44:31.652Z}\n  }#215A957565E17E4AEE20CB3E1667FBF15697A140\n}#CF6F909CF3EE0316723FE043F0E50F6D0F4C166C"),
		Debug(false))
	if err != nil {
		fmt.Println(err)
	}
	assert.Nil(t, err)
	j, err := json.MarshalIndent(result, "", "  ")
	assert.Nil(t, err)
	fmt.Println(string(j))
}

func TestParser1(t *testing.T) {
	result, err := Parse("Parser1", []byte("Block{\nValue: 12\n}#"), Debug(false))
	if err != nil {
		fmt.Println(err)
	}
	assert.Nil(t, err)
	j, err := json.MarshalIndent(result, "", "  ")
	assert.Nil(t, err)
	fmt.Println(string(j))
}
