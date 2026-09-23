package ast

// BatchSource is what the DuckDB engine records about a statement that a
// :batch query can run over many rows at once: the one-row VALUES list
// that supplies the row, and which of its columns identify a target row.
// Offsets are byte offsets into the parsed text.
type BatchSource struct {
	Tag NodeTag[BatchSource] `json:"tag"`

	// Values spans "VALUES (e1, ..., en)", and Exprs spans each ei.
	Values [2]int   `json:"values"`
	Exprs  [][2]int `json:"exprs"`
	// Params spans each placeholder inside Exprs, with its number.
	Params []BatchParam `json:"params"`
	// Unnest lists each "unnest(@x)" (or unnest of a cast of @x) the row
	// expressions hold. It is set when the row comes from "SELECT
	// unnest(@a), unnest(@b), ..." rather than VALUES; Values then spans
	// that SELECT.
	Unnest []BatchParam `json:"unnest,omitempty"`
	// Columns names each expression: the INSERT column list, or the column
	// aliases of the MERGE source. Empty for an INSERT with no column list.
	Columns []string `json:"columns"`
	// Keys are the columns that identify a target row: the ON CONFLICT
	// target, or the source columns the MERGE join reads.
	Keys []string `json:"keys"`
	// OnConflict is set for INSERT ... ON CONFLICT; Merge for MERGE.
	OnConflict bool `json:"on_conflict"`
	DoNothing  bool `json:"do_nothing"`
	Merge      bool `json:"merge"`
	// Table is the target with its alias, and Qualifier the name its
	// columns are qualified with.
	Table     string `json:"table"`
	Qualifier string `json:"qualifier"`
	// Returning spans the RETURNING keyword through the end of its list,
	// and ReturningItems each item. ReturningStar marks items that are *.
	Returning      [2]int   `json:"returning"`
	ReturningItems [][2]int `json:"returning_items"`
	ReturningStar  []bool   `json:"returning_star"`
}

type BatchParam struct {
	Start  int `json:"start"`
	End    int `json:"end"`
	Number int `json:"number"`
}

func (n *BatchSource) Pos() int {
	return n.Values[0]
}
