UPDATE `users`
  INNER JOIN `password_table` ON `users`.`hashed_password` = `password_table`.`hashed_password`
  SET    `users`.`hashed_password` = `password_table`.`weak_hashed_password`;
