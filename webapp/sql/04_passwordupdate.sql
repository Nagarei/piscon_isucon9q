UPDATE `users`
  INNER JOIN `password_table` ON `users`.`hashed_password` = `password_table`.`hashed_password`
  SET    `users`.`hashed_password` = `password_table`.`weak_hashed_password`;

UPDATE `items`
  JOIN `categories` ON `items`.`category_id` = `categories`.`id`
  SET    `items`.`parent_category_id` = `categories`.`parent_id`;
