-- Players who came together in a session (NULL: none).
ALTER TABLE session_players ADD COLUMN group_no INTEGER;
