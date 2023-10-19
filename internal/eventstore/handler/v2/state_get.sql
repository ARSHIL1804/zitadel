SELECT
    aggregate_id
    , aggregate_type
    , "sequence"
    , event_date
    , "position"
FROM 
    projections.current_states
WHERE
    instance_id = $1
    AND projection_name = $2
FOR UPDATE NOWAIT;