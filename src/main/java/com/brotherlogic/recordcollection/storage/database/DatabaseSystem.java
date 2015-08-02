package com.brotherlogic.recordcollection.storage.database;

import java.sql.Connection;
import java.sql.SQLException;

public class DatabaseSystem {

  private Database first = new DatabaseV1();
  
  public DatabaseSystem(Database init) {
    first = init;
  }
  
  public void initDatabase(Connection con) throws SQLException {
    if (!first.validate(con))
      first.create(con);
  }

  public void cleanDatabase(Connection con) throws SQLException {
    if (first.validate(con))
      first.destroy(con);
  }
  
}
