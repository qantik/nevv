package service

import (
	"crypto/cipher"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/ed25519"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"

	"github.com/qantik/nevv/api"
)

var suite abstract.Suite
var stream cipher.Stream

func init() {
	suite = ed25519.NewAES128SHA256Ed25519(false)
	stream = suite.Cipher(abstract.RandomKey)

}

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestPing(t *testing.T) {
	local := onet.NewTCPTest()

	hosts, _, _ := local.GenTree(3, true)
	defer local.CloseAll()

	services := castServices(local.GetServices(hosts, serviceID))

	ping := &api.Ping{0}

	p1, _ := services[0].Ping(ping)
	p2, _ := services[1].Ping(ping)
	p3, _ := services[2].Ping(ping)

	assert.Equal(t, uint32(1), p1.Nonce, p2.Nonce, p3.Nonce)
}

func TestGenerateElection(t *testing.T) {
	local := onet.NewTCPTest()

	hosts, roster, _ := local.GenTree(3, true)
	defer local.CloseAll()

	services := castServices(local.GetServices(hosts, serviceID))

	election := api.Election{"test", "", "", "", []byte{}, roster, []string{}, nil, ""}
	msg := &api.GenerateElection{Token: "", Election: election}

	response, err := services[0].GenerateElection(msg)
	if err != nil {
		log.ErrFatal(err)
	}

	<-time.After(250 * time.Millisecond)

	key1 := services[0].Storage.Chains["test"].Election().Key
	key2 := services[1].Storage.Chains["test"].Election().Key
	key3 := services[2].Storage.Chains["test"].Election().Key

	assert.Equal(t, key1, key2, key3, response.Key)
}

func TestGetElections(t *testing.T) {
	local := onet.NewTCPTest()

	hosts, roster, _ := local.GenTree(3, true)
	defer local.CloseAll()

	services := castServices(local.GetServices(hosts, serviceID))

	election1 := api.Election{"e1", "", "", "", []byte{}, roster, []string{"u1"}, nil, ""}
	election2 := api.Election{"e2", "admin", "", "", []byte{}, roster, []string{}, nil, ""}

	ge := &api.GenerateElection{Token: "", Election: election1}
	_, _ = services[0].GenerateElection(ge)
	ge = &api.GenerateElection{Token: "", Election: election2}
	_, _ = services[0].GenerateElection(ge)

	ger, err := services[0].GetElections(&api.GetElections{"", "u2"})
	if err != nil {
		log.ErrFatal(err)
	}
	assert.Equal(t, 0, len(ger.Elections))

	ger, err = services[1].GetElections(&api.GetElections{"", "admin"})
	if err != nil {
		log.ErrFatal(err)
	}
	assert.Equal(t, 1, len(ger.Elections))
	assert.Equal(t, "admin", ger.Elections[0].Admin)

	ger, err = services[2].GetElections(&api.GetElections{"", "u1"})
	if err != nil {
		log.ErrFatal(err)
	}
	assert.Equal(t, 1, len(ger.Elections))
	assert.Equal(t, "u1", ger.Elections[0].Users[0])
}

func TestCastBallot(t *testing.T) {
	election, services, local := newElection()
	defer local.CloseAll()

	ge := &api.GenerateElection{Token: "", Election: *election}
	response, _ := services[0].GenerateElection(ge)

	<-time.After(250 * time.Millisecond)

	alpha, beta := encrypt(suite, response.Key, []byte{1, 2, 3})

	ballot := api.Ballot{"user", alpha, beta, []byte{}}
	cb := &api.CastBallot{"", "test", ballot}

	cbr, err := services[0].CastBallot(cb)
	if err != nil {
		log.ErrFatal(err)
	}

	assert.Equal(t, uint32(2), cbr.Block)

	ballots1, _ := services[0].Storage.Chains["test"].Ballots()
	ballots2, _ := services[1].Storage.Chains["test"].Ballots()
	ballots3, _ := services[2].Storage.Chains["test"].Ballots()

	assert.Equal(t, ballots1[0], ballots2[0], ballots3[0])
}

func TestGetBallots(t *testing.T) {
	election, services, local := newElection()
	defer local.CloseAll()

	ge := &api.GenerateElection{Token: "", Election: *election}
	response, _ := services[0].GenerateElection(ge)

	<-time.After(250 * time.Millisecond)

	alpha1, beta1 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot1 := api.Ballot{"user1", alpha1, beta1, []byte{}}
	alpha2, beta2 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot2 := api.Ballot{"user2", alpha2, beta2, []byte{}}
	alpha3, beta3 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot3 := api.Ballot{"user2", alpha3, beta3, []byte{}}
	alpha4, beta4 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot4 := api.Ballot{"user3", alpha4, beta4, []byte{}}

	_, _ = services[0].CastBallot(&api.CastBallot{"", "test", ballot1})
	_, _ = services[1].CastBallot(&api.CastBallot{"", "test", ballot2})
	_, _ = services[2].CastBallot(&api.CastBallot{"", "test", ballot3})
	_, _ = services[0].CastBallot(&api.CastBallot{"", "test", ballot4})

	gbr, err := services[0].GetBallots(&api.GetBallots{"", "test"})
	if err != nil {
		log.ErrFatal(err)
	}

	assert.Equal(t, 3, len(gbr.Ballots))
}

func TestShuffle(t *testing.T) {
	election, services, local := newElection()
	defer local.CloseAll()

	ge := &api.GenerateElection{Token: "", Election: *election}
	response, _ := services[0].GenerateElection(ge)

	<-time.After(250 * time.Millisecond)

	alpha1, beta1 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot1 := api.Ballot{"user1", alpha1, beta1, []byte{}}
	alpha2, beta2 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot2 := api.Ballot{"user2", alpha2, beta2, []byte{}}

	_, _ = services[0].CastBallot(&api.CastBallot{"", "test", ballot1})
	_, _ = services[1].CastBallot(&api.CastBallot{"", "test", ballot2})

	shr, err := services[0].Shuffle(&api.Shuffle{"", "test"})
	if err != nil {
		log.ErrFatal(err)
	}

	assert.Equal(t, 4, int(shr.Block))
}

func TestGetShuffle(t *testing.T) {
	election, services, local := newElection()
	defer local.CloseAll()

	ge := &api.GenerateElection{Token: "", Election: *election}
	response, _ := services[0].GenerateElection(ge)

	<-time.After(250 * time.Millisecond)

	_, err := services[0].GetShuffle(&api.GetShuffle{"", "test"})
	assert.NotNil(t, err)

	alpha1, beta1 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot1 := api.Ballot{"user1", alpha1, beta1, []byte{}}
	alpha2, beta2 := encrypt(suite, response.Key, []byte{1, 2, 3})
	ballot2 := api.Ballot{"user2", alpha2, beta2, []byte{}}

	_, _ = services[0].CastBallot(&api.CastBallot{"", "test", ballot1})
	_, _ = services[1].CastBallot(&api.CastBallot{"", "test", ballot2})

	_, _ = services[0].Shuffle(&api.Shuffle{"", "test"})

	gsr, _ := services[0].GetShuffle(&api.GetShuffle{"", "test"})
	assert.Equal(t, 2, len(gsr.Box.Ballots))
}

func TestDecrypt(t *testing.T) {
	election, services, local := newElection()
	defer local.CloseAll()

	ge := &api.GenerateElection{Token: "", Election: *election}

	response, _ := services[0].GenerateElection(ge)

	<-time.After(250 * time.Millisecond)

	alpha1, beta1 := encrypt(suite, response.Key, []byte("user1"))
	ballot1 := api.Ballot{"user1", alpha1, beta1, []byte{}}
	alpha2, beta2 := encrypt(suite, response.Key, []byte("user2"))
	ballot2 := api.Ballot{"user2", alpha2, beta2, []byte{}}
	alpha3, beta3 := encrypt(suite, response.Key, []byte("user3"))
	ballot3 := api.Ballot{"user3", alpha3, beta3, []byte{}}

	_, _ = services[0].CastBallot(&api.CastBallot{"", "test", ballot1})
	_, _ = services[1].CastBallot(&api.CastBallot{"", "test", ballot2})
	_, _ = services[2].CastBallot(&api.CastBallot{"", "test", ballot3})

	_, _ = services[0].Shuffle(&api.Shuffle{"", "test"})

	dr, err := services[0].Decrypt(&api.Decrypt{"", "test"})
	if err != nil {
		log.ErrFatal(err)
	}
	assert.Equal(t, uint32(6), dr.Block)

	boxes, _ := services[2].Storage.Chains["test"].Boxes()
	assert.Equal(t, 2, len(boxes))

	assert.Equal(t, boxes[1].Ballots[0].User, string(boxes[1].Ballots[0].Clear))
	assert.Equal(t, boxes[1].Ballots[1].User, string(boxes[1].Ballots[1].Clear))
	assert.Equal(t, boxes[1].Ballots[2].User, string(boxes[1].Ballots[2].Clear))
}

func castServices(services []onet.Service) []*Service {
	cast := make([]*Service, len(services))
	for i, service := range services {
		cast[i] = service.(*Service)
	}

	return cast
}

func encrypt(suite abstract.Suite, pub abstract.Point, msg []byte) (K, C abstract.Point) {
	M, _ := suite.Point().Pick(msg, random.Stream)

	k := suite.Scalar().Pick(random.Stream)
	K = suite.Point().Mul(nil, k)
	S := suite.Point().Mul(pub, k)
	C = S.Add(S, M)

	return
}

func newElection() (*api.Election, []*Service, *onet.LocalTest) {
	local := onet.NewTCPTest()

	hosts, roster, _ := local.GenTree(3, true)
	services := castServices(local.GetServices(hosts, serviceID))
	election := &api.Election{"test", "", "", "", []byte{}, roster, []string{}, nil, ""}

	return election, services, local
}