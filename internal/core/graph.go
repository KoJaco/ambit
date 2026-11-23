package core

// GraphNode is the compact node row returned by Graph.
type GraphNode struct {
	ID             NodeID `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Parent         NodeID `json:"parent,omitempty"`
	RelationshipID RelID  `json:"relationship_id,omitempty"`
	Status         Status `json:"status"`
	// DependsOn lists build-order dependencies (omitted when none).
	DependsOn []NodeID `json:"depends_on,omitempty"`
}

// GraphEdge is one relationship row returned by Graph.
type GraphEdge struct {
	ID    RelID  `json:"id"`
	From  NodeID `json:"from"`
	To    NodeID `json:"to"`
	Label string `json:"label,omitempty"`
	Kind  string `json:"kind,omitempty"`
	// HasContract is true when the relationship carries a non-empty contract.
	HasContract bool `json:"has_contract,omitempty"`
}

// GraphView is the get_graph payload: the graph plus the active method and stage.
type GraphView struct {
	Method string      `json:"method"`
	Stage  string      `json:"stage,omitempty"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
}

// Graph returns every node (compact fields, no spec) and relationship in model order.
func (idx *Index) Graph() ([]GraphNode, []GraphEdge) {
	v := idx.GraphView()
	return v.Nodes, v.Edges
}

// GraphView is Graph plus the active method id and effective stage, read in one snapshot.
func (idx *Index) GraphView() GraphView {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	nodes := make([]GraphNode, 0, len(idx.Order))
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		nodes = append(nodes, GraphNode{ID: n.ID, Name: n.Name, Type: n.Type, Parent: n.ParentID, RelationshipID: n.RelationshipID, Status: n.Status, DependsOn: append([]NodeID(nil), n.DependsOn...)})
	}
	edges := make([]GraphEdge, 0, len(idx.Relationships))
	for _, r := range idx.Relationships {
		edges = append(edges, GraphEdge{ID: r.ID, From: r.From, To: r.To, Label: r.Label, Kind: r.Kind, HasContract: !r.Contract.IsEmpty()})
	}
	return GraphView{Method: idx.activeMethodLocked().ID, Stage: idx.effectiveStageLocked(), Nodes: nodes, Edges: edges}
}
