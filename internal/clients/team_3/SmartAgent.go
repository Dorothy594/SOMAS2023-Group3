package team_3

import (
	"SOMAS2023/internal/common/objects"
	"SOMAS2023/internal/common/physics"
	"SOMAS2023/internal/common/utils"
	"SOMAS2023/internal/common/voting"
	"math"
	"math/rand"

	"sort"

	"github.com/google/uuid"
)

type ISmartAgent interface {
	objects.IBaseBiker
}

type SmartAgent struct {
	objects.BaseBiker
	currentBike   *objects.MegaBike
	targetLootBox objects.ILootBox
	reputationMap map[uuid.UUID]reputation

	lootBoxCnt      float64
	energySpent     float64
	lastEnergyLevel float64
}

// DecideAction only pedal
func (agent *SmartAgent) DecideAction() objects.BikerAction {
	if agent.GetEnergyLevel() < agent.lastEnergyLevel {
		agent.energySpent += agent.lastEnergyLevel - agent.GetEnergyLevel()
	}
	agent.lastEnergyLevel = agent.GetEnergyLevel()

	agent.updateRepMap()
	return objects.Pedal
}

// DecideForces randomly based on current energyLevel
func (agent *SmartAgent) DecideForces(direction uuid.UUID) {
	energyLevel := agent.GetEnergyLevel() // 当前能量

	pedalForce := rand.Float64() * energyLevel // 使用 rand 包生成随机的 pedal 力量，可以根据需要调整范围

	// 因为force是一个struct,包括pedal, brake,和turning，因此需要一起定义，不能够只有pedal
	forces := utils.Forces{
		Pedal: pedalForce,
		Brake: 0.0, // 这里默认刹车为 0
		Turning: utils.TurningDecision{
			SteerBike:     true,
			SteeringForce: physics.ComputeOrientation(agent.GetLocation(), agent.GetGameState().GetMegaBikes()[direction].GetPosition()) - agent.GetGameState().GetMegaBikes()[agent.GetMegaBikeId()].GetOrientation(),
		}, // 这里默认转向为 0
	}

	agent.SetForces(forces)
}

// DecideJoining accept all
func (agent *SmartAgent) DecideJoining(pendingAgents []uuid.UUID) map[uuid.UUID]bool {
	decision := make(map[uuid.UUID]bool)
	for _, agent := range pendingAgents {
		decision[agent] = true
	}
	return decision
}

func (agent *SmartAgent) ProposeDirection() utils.Coordinates {
	e := agent.decideTargetLootBox(agent.GetGameState().GetLootBoxes())
	if e != nil {
		panic("unexpected error!")
	}
	return agent.targetLootBox.GetPosition()
}

func (agent *SmartAgent) FinalDirectionVote(proposals []uuid.UUID) voting.LootboxVoteMap {
	boxesInMap := agent.GetGameState().GetLootBoxes()
	boxProposed := make([]objects.ILootBox, len(proposals))
	for i, pp := range proposals {
		boxProposed[i] = boxesInMap[pp]
	}
	rank, _ := agent.rankTargetProposals(boxProposed)
	return rank
}

func (agent *SmartAgent) DecideAllocation() voting.IdVoteMap {
	agent.lootBoxCnt += 1
	currentBike := agent.GetGameState().GetMegaBikes()[agent.GetMegaBikeId()]
	vote, _ := agent.scoreAgentsForAllocation(currentBike.GetAgents())
	return vote
}

func (agent *SmartAgent) find_same_colour_highest_loot_lootbox(proposedLootBox []objects.ILootBox) error {
	max_loot := 0.0
	for _, lootbox := range proposedLootBox {
		loot := lootbox.GetTotalResources()
		if loot > max_loot {
			max_loot = loot
			agent.targetLootBox = lootbox
		}
	}
	return nil
}

func (agent *SmartAgent) find_leader(agentsOnBike []objects.IBaseBiker, proposedLootBox []objects.ILootBox) (map[uuid.UUID]float64, error) {
	// two-round run-off

	// the first round: top three
	scores := make(map[uuid.UUID]float64)

	for _, others := range agentsOnBike {
		id := others.GetID()
		if id != agent.GetID():
			rep := agent.reputationMap[id]
			score := rep.historyContribution + rep.lootBoxGet/ // Pareto principle: give more energy to those with more outcome
				+rep.isSameColor/ // Cognitive dimension: is same belief?
				+rep.energyRemain // necessity: must stay alive
	
			scores[id] = score
	}

	sortedIDs := make([]uuid.UUID, 0, len(scores))
	for score := range scores {
		sortedIDs = append(sortedIDs, score)
	}

	sort.Slice(sortedIDs, func(i, j int) bool {
		return scores[sortedIDs[i]] > scores[sortedIDs[j]]
	})

	var topThree []uuid.UUID
	if len(sortedIDs) >= 3 {
		topThree = sortedIDs[:3]
	} else {
		topThree = sortedIDs
	}

	// the second round: borda count
	scores2 := make([]float64, 0)
	for _, uuid := range topThree {
		rep := agent.reputationMap[uuid]
		score := rep.recentContribution // Forgiveness: forgive agents pedal harder recently
		scores2 = append(scores2, score)
	}

	elementCount := make(map[float64]int)
	for _, num := range scores2 {
		elementCount[num]++
	}
	uniqueElements := make([]float64, 0, len(elementCount))
	for num := range elementCount {
		uniqueElements = append(uniqueElements, num)
	}
	sort.Float64s(uniqueElements)
	elementOrder := make(map[float64]int)
	for i, num := range uniqueElements {
		elementOrder[num] = i + 1
	}
	elementOrderList := make([]int, len(scores))
	for i, num := range scores2 {
		elementOrderList[i] = elementOrder[num]
	}

	rank := make(map[uuid.UUID]float64)
	for i, lootBox := range proposedLootBox {
		rank[lootBox.GetID()] = float64(elementOrderList[i])
	}

	return rank, nil // prepare for borda count
}

func (agent *SmartAgent) other_agents_strong(agentsOnBike []objects.IBaseBiker, proposedLootBox []objects.ILootBox) bool {
	// other_agents' energy is higher than the farthest lootbox

	// other_agents' energy
	other_agents_energy := 0.0
	for _, others := range agentsOnBike {
		other_agents_energy += others.GetEnergyLevel()
	}

	//farthest lootbox
	max_distance := 0.0
	for _, lootbox := range proposedLootBox {
		distance := physics.ComputeDistance(lootbox.GetPosition(), agent.GetLocation())
		if distance > float64(max_distance) {
			max_distance = distance
		}
	}
	max_energy := max_distance * 1

	return other_agents_energy > max_energy
}

func (agent *SmartAgent) all_weak(agentsOnBike []objects.IBaseBiker, proposedLootBox []objects.ILootBox) bool {
	total_energy := 0.0

	// total_energy
	for _, others := range agentsOnBike {
		total_energy += others.GetEnergyLevel()
	}

	// nearest same_colour lootbox
	nearest_same_colour_lootbox_distance := math.MaxFloat64
	for _, lootbox := range proposedLootBox {
		if lootbox.GetColour() == agent.GetColour() {
			distance := physics.ComputeDistance(lootbox.GetPosition(), agent.GetLocation())
			if distance < nearest_same_colour_lootbox_distance {
				nearest_same_colour_lootbox_distance = distance
			}
		}
	}
	nearest_same_colour_lootbox_energy := nearest_same_colour_lootbox_distance * 1

	return total_energy < nearest_same_colour_lootbox_energy
}

func (agent *SmartAgent) find_closest_lootbox(proposedLootBox []objects.ILootBox) error {
	min_distance := math.MaxFloat64

	for _, lootbox := range proposedLootBox {
		distance := physics.ComputeDistance(lootbox.GetPosition(), agent.GetLocation())
		// no need to normalize
		if distance < min_distance {
			min_distance = distance
			agent.targetLootBox = lootbox
		}
	}
	return nil
}

func (agent *SmartAgent) decideTargetLootBox(agentsOnBike []objects.IBaseBiker, proposedLootBox []objects.ILootBox) error {
	max_score := 0.0

	if agent.all_weak(agentsOnBike, proposedLootBox) == true { //all weak
		agent.find_closest_lootbox(proposedLootBox)
	}

	if agent.other_agents_strong(agentsOnBike, proposedLootBox) == true { //is strong
		agent.find_same_colour_highest_loot_lootbox(proposedLootBox)
	}

	for _, lootbox := range proposedLootBox {
		// agent
		loot := (lootbox.GetTotalResources() / 8.0) //normalize
		is_color := 0.0
		if lootbox.GetColour() == agent.GetColour() {
			is_color = 1.0
		}
		distance := physics.ComputeDistance(lootbox.GetPosition(), agent.GetLocation())
		normalized_distance := distance / ((utils.GridHeight) * (utils.GridWidth))
		score := 0.2*loot + 0.2*is_color + (-0.3)*normalized_distance

		// environment
		same_colour_bikers := make([]objects.IBaseBiker, 0)
		same_colour := 0
		for _, others := range agentsOnBike {
			if pthers.GetColour() == lootbox.GetColour() {
				same_colour += 1
				same_colour_bikers = append(same_colour_bikers, others)
			}
			score += 0.5 * float64(same_colour/len(agentsOnBike))
		}

		for _, others := range same_colour_bikers {
			id := others.GetID()
			rep := agent.reputationMap[id]
			score += 0.5 * 0.4 * rep.historyContribution /
				+0.5 * 0.2 * rep.recentContribution /
				+0.5 * 0.4 * rep.energyRemain
		}

		if score > max_score {
			max_score = score
			agent.targetLootBox = lootbox
		}
	}
	return nil
}

func (agent *SmartAgent) rankTargetProposals(proposedLootBox []objects.ILootBox) (map[uuid.UUID]float64, error) {

	scores := make([]float64, 0)
	for _, lootbox := range proposedLootBox {
		loot := (lootbox.GetTotalResources() / 8.0)
		is_color := 0.0
		if lootbox.GetColour() == agent.GetColour() {
			is_color = 1.0
		}
		// 如何从lootbox,知道它是由哪个agent提出的,从而去考察这些agents的reputation
		// agents_rep 这个点还不知道

		distance := physics.ComputeDistance(lootbox.GetPosition(), agent.GetLocation())
		normalized_distance := distance / ((utils.GridHeight) * (utils.GridWidth))
		score := 0.2*loot + 0.2*is_color + 0.2*normalized_distance // + 0.4*agents_rep

		scores = append(scores, score)
	}
	 // We choose to use the Borda count method to pick a proposal because it can mitigate the Condorcet paradox.
         // Borda count needs to get the rank of all candidates to score Borda points.
         // In this case, according to the Gibbard-Satterthwaite Theorem, Borda count is susceptible to tactical voting.
         // The following steps tend to achieve the rank of lootbox proposals according to their scores calculated. We will return the highest rank to pick the agent with it. (Another Borda score would consider reputation function)这个后面如果可以再考虑如果能得到的话

	elementCount := make(map[float64]int)
	for _, num := range scores {
		elementCount[num]++
	}
	uniqueElements := make([]float64, 0, len(elementCount))
	for num := range elementCount {
		uniqueElements = append(uniqueElements, num)
	}
	sort.Float64s(uniqueElements)
	elementOrder := make(map[float64]int)
	for i, num := range uniqueElements {
		elementOrder[num] = i + 1
	}
	elementOrderList := make([]int, len(scores))
	for i, num := range scores {
		elementOrderList[i] = elementOrder[num]
	}

	rank := make(map[uuid.UUID]float64)
	for i, lootBox := range proposedLootBox {
		rank[lootBox.GetID()] = float64(elementOrderList[i])
	}

	return rank, nil // prepare for borda count
}

// rankAgentReputation if self energy level is low (below average cost for a lootBox), we follow 'Smallest First', else 'Ration'
func (agent *SmartAgent) scoreAgentsForAllocation(agentsOnBike []objects.IBaseBiker) (map[uuid.UUID]float64, error) {
	scores := make(map[uuid.UUID]float64)
	totalScore := 0.0
	if agent.energySpent/agent.lootBoxCnt > agent.GetEnergyLevel() {
		// go 'Smallest First' strategy, only take energyRemain into consideration
		for _, others := range agentsOnBike {
			id := others.GetID()
			score := agent.reputationMap[id].energyRemain
			scores[others.GetID()] = score
			totalScore += score
		}
	} else {
		// go 'Ration' strategy, considering all facts
		for _, others := range agentsOnBike {
			id := others.GetID()
			rep := agent.reputationMap[id]
			score := rep.isSameColor/ // Cognitive dimension: is same belief?
				+rep.historyContribution + rep.lootBoxGet/ // Pareto principle: give more energy to those with more outcome
				+rep.recentContribution/ // Forgiveness: forgive agents pedal harder recently
				-rep.energyGain/ // Equality: Agents received more energy before should get less this time
				+rep.energyRemain // Need: Agents with lower energyLevel require more, try to meet their need
			scores[others.GetID()] = score
			totalScore += score
		}
	}

	// normalize scores
	for id, score := range scores {
		scores[id] = score / totalScore
	}

	return scores, nil
}

func (agent *SmartAgent) updateRepMap() {
	if agent.reputationMap == nil {
		agent.reputationMap = make(map[uuid.UUID]reputation)
	}
	for _, bikes := range agent.GetGameState().GetMegaBikes() {
		for _, otherAgent := range bikes.GetAgents() {
			rep, exist := agent.reputationMap[otherAgent.GetID()]
			if !exist {
				rep = reputation{}
			}
			rep.updateScore(otherAgent, agent.GetColour())
			agent.reputationMap[otherAgent.GetID()] = rep
		}
	}
}
