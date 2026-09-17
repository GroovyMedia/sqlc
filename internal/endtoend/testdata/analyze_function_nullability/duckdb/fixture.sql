INSERT INTO authors VALUES
  (1, 'Ann', 'writes', '1970-01-02', true, '1', union_value(num := 1)),
  (2, 'Bob', NULL, NULL, false, '"two"', union_value(str := 'bob')),
  (3, 'Cid', 'draws', '1980-03-04', true, '3', union_value(num := 3));
