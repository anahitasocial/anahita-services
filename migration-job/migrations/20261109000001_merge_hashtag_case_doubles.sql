-- Merges hashtags that differ only in their capitals.
--
-- hashtag-service looked a post's hashtags up in the database, which ignores
-- case, and then matched what came back in code that did not: a post from
-- another server writing #Anahita found #anahita and made a second hashtag
-- beside it. Fourteen were made so on the development database on
-- 2026-10-13, all from federated posts, splitting a hashtag's page and its
-- counts in two. The service now matches without regard to case and keeps a
-- hashtag's address (alias) in lower case.
--
-- Each double that has a lower-case original is merged into it: its posts
-- are tagged with the original, and it is removed. One with no original
-- keeps its name, as it is shown, and has its address put in lower case, so
-- that the next post finds it.
--
-- Idempotent: a second run finds no alias with capitals in it. Only
-- federation made such hashtags, so legacy/upgrade.sql has none to repair.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_merge_hashtag_case_doubles()
BEGIN
    DROP TEMPORARY TABLE IF EXISTS hashtag_doubles;
    CREATE TEMPORARY TABLE hashtag_doubles (
        double_id BIGINT UNSIGNED PRIMARY KEY,
        original_id BIGINT UNSIGNED NULL
    );

    INSERT INTO hashtag_doubles (double_id, original_id)
    SELECT d.id,
           (SELECT MIN(o.id)
              FROM nodes AS o
             WHERE o.type = d.type
               AND o.id <> d.id
               AND BINARY o.alias = LOWER(d.alias))
      FROM nodes AS d
     WHERE d.type = 'node.tag.hashtag-service.hashtag.v1'
       AND BINARY d.alias <> LOWER(d.alias);

    -- The double's edges become the original's. One the original has
    -- already (the same post tagged with both spellings) collides with the
    -- unique key and is left, to be removed below.
    UPDATE IGNORE edges AS e
      JOIN hashtag_doubles AS h ON h.double_id = e.node_a_id
       SET e.node_a_id = h.original_id
     WHERE h.original_id IS NOT NULL;
    UPDATE IGNORE edges AS e
      JOIN hashtag_doubles AS h ON h.double_id = e.node_b_id
       SET e.node_b_id = h.original_id
     WHERE h.original_id IS NOT NULL;

    DELETE e
      FROM edges AS e
      JOIN hashtag_doubles AS h ON h.double_id IN (e.node_a_id, e.node_b_id)
     WHERE h.original_id IS NOT NULL;

    DELETE n
      FROM nodes AS n
      JOIN hashtag_doubles AS h ON h.double_id = n.id
     WHERE h.original_id IS NOT NULL;

    -- No original: the address in lower case, the name as it was.
    UPDATE nodes AS n
      JOIN hashtag_doubles AS h ON h.double_id = n.id
       SET n.alias = LOWER(n.alias)
     WHERE h.original_id IS NULL;

    DROP TEMPORARY TABLE hashtag_doubles;
END;
-- +goose StatementEnd
CALL anahita_merge_hashtag_case_doubles();
DROP PROCEDURE anahita_merge_hashtag_case_doubles;

-- +goose Down
-- +goose StatementBegin
-- Not reversible: the doubles are the defect, and putting them back would
-- split each hashtag in two again.
SELECT 1;
-- +goose StatementEnd
