UPDATE `users`
  INNER JOIN `password_table` ON `users`.`hashed_password` = `password_table`.`hashed_password`
  SET    `users`.`hashed_password` = `password_table`.`weak_hashed_password`;


UPDATE `items`
  SET  `parent_category_id` = (IF( `category_id` < 10, IF(`category_id` = 1, 0, 1), IF(`category_id` % 10 = 0, 0, (`category_id` DIV 10)*10)));

