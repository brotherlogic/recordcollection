package com.brotherlogic.recordcollection.storage.database;

import java.sql.Connection;
import java.sql.SQLException;

public interface Database {

  void create(Connection con) throws SQLException;
  void destroy(Connection con) throws SQLException;
  void upgrade(Connection con) throws SQLException;
  boolean validate(Connection con) throws SQLException;
  Database getNextVersion();
  
}
