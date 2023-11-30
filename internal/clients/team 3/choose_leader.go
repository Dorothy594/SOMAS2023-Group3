// 在每轮结束时调用，选择一个 leader
func (agent *BaselineAgent) ChooseLeader(agentsOnBike []objects.BaseBiker) (uuid.UUID, bool) {
	// 第一轮投票，选择前三名候选人
	leaderCandidates, err := agent.getTopReputationCandidates(agentsOnBike)
	if err != nil {
		panic("unexpected error!")
	}

	// 使用 Borda Count 进行第一轮投票
	firstRoundWinner, firstRoundVotes := agent.runBordaCount(leaderCandidates)

	// 第二轮投票，选择第一轮获胜者
	secondRoundVotes := agent.runFinalVote(firstRoundWinner, agentsOnBike)

	// 检查是否超过半数
	if countYesVotes(secondRoundVotes) > len(agentsOnBike)/2 {
		return firstRoundWinner, true // 成功当选 leader
	}

	return uuid.Nil, false // 未成功当选 leader，uuid.Nil就是没有任何人成为leader
}

// 获取前三名声望最高的候选人
func (agent *BaselineAgent) getTopReputationCandidates(agentsOnBike []objects.BaseBiker) ([]uuid.UUID, error) {
	// 获取每个 agentsOnBike 的reputation
	reputationRank, err := agent.rankAgentsReputation(agentsOnBike)
	if err != nil {
		return nil, err
	}

	// 根据声望排名选择前三名候选人
	var topCandidates []uuid.UUID
	for _, id := range agent.getTopNKeys(reputationRank, 3) {
		topCandidates = append(topCandidates, id)
	}

	return topCandidates, nil
}

// Borda Count 投票方法
func (agent *BaselineAgent) runBordaCount(candidates []uuid.UUID) (uuid.UUID, map[uuid.UUID]int) {
	votes := make(map[uuid.UUID]int)

	// 不是每个agent, 而是整体, 整辆车对所有agents进行排序
	for _, agentID := range candidates {
		rank := agent.simulateRanking(candidates)
		votes[agentID] = rank
	}

	// 计算总分数，选择最高分者
	winner := agent.findBordaWinner(votes)
	return winner, votes
}

// 最终投票，判断是否超过半数
func (agent *BaselineAgent) runFinalVote(winner uuid.UUID, agentsOnBike []objects.BaseBiker) map[uuid.UUID]bool {
	votes := make(map[uuid.UUID]bool)
	// 这里的map[uuid.UUID]bool,
	// 1-yes, 2-no, 3-yes, 4-yes, 5-no, votes = ['yes','no','yes','yes','no']

	// 所有代理投票
	for _, agentID := range agent.getAllAgentIDs(agentsOnBike) {
		vote := agent.simulateFinalVote(winner)
		votes[agentID] = vote
	}
	return votes
}

// 选择声望排名最高的前 N 名
func (agent *BaselineAgent) getTopNKeys(reputation map[uuid.UUID]float64, n int) []uuid.UUID {
	// 这里是针对整体而言的，不应该写在BaselineAgent里
	return nil
}

// agent如何做排名
func (agent *BaselineAgent) simulateRanking(candidates []uuid.UUID) int {
	// TODO: 实现模拟代理的排名逻辑，可以随机生成
	return rand.Intn(len(candidates) + 1)
}

// 查找 Borda Count 结果中的获胜者
func (agent *BaselineAgent) findBordaWinner(votes map[uuid.UUID]int) uuid.UUID {
	// TODO: 实现查找 Borda Count 结果中的获胜者逻辑
	return uuid.Nil
}