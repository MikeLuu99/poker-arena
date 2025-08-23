package models

type Player struct {
	Name  string   `json:"name"`
	Chips int      `json:"chips"`
	Cards []string `json:"cards"`
	Model string   `json:"model"`
}

type GameState struct {
	Players           []Player       `json:"players"`
	Deck              []string       `json:"deck"`
	CommunityCards    []string       `json:"communityCards"`
	Pot               int            `json:"pot"`
	CurrentPlayer     int            `json:"currentPlayer"`
	Round             string         `json:"round"`
	HandNumber        int            `json:"handNumber"`
	GameLog           []string       `json:"gameLog"`
	CurrentBet        int            `json:"currentBet"`
	PlayerBets        map[string]int `json:"playerBets"`
	LastRaiseAmount   int            `json:"lastRaiseAmount"`
	MinRaise          int            `json:"minRaise"`
	FoldedPlayers     []string       `json:"foldedPlayers"`
	DealerPosition    int            `json:"dealerPosition"`
	SmallBlind        int            `json:"smallBlind"`
	BigBlind          int            `json:"bigBlind"`
	BettingComplete   bool           `json:"bettingComplete"`
	EliminatedPlayers []string       `json:"eliminatedPlayers"`
	GameEnded         bool           `json:"gameEnded"`
}

type GameResult struct {
	Winner        Player   `json:"winner"`
	TotalHands    int      `json:"totalHands"`
	AllPlayers    []Player `json:"allPlayers"`
	Eliminated    []string `json:"eliminated"`
	FinalChips    int      `json:"finalChips"`
	GameDuration  string   `json:"gameDuration"`
}