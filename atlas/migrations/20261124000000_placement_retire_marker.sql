-- Track the verified replica that an in-flight capacity rebalance will remove
-- once a replacement copy has been verified on a less-used node.
-- NULL means no move is in flight for that placement.
ALTER TABLE placements ADD COLUMN retire_requested_at timestamptz;
