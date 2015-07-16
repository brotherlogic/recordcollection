package com.brotherlogic.recordcollection.storage.database;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;

public class DatabaseV1 implements Database {

  private Logger logger = Logger.getLogger(getClass());
  
  @Override
  public void create(Connection con) throws SQLException {
    Statement s = con.createStatement();
    s.execute("CREATE TABLE key_table (key char(40) PRIMARY KEY, secret char(40))");
  }

  @Override
  public void destroy(Connection con) throws SQLException {
    Statement s = con.createStatement();
    s.execute("DROP TABLE key_table");
  }

  @Override
  public void upgrade(Connection con) throws SQLException {
    create(con);
  }

  @Override
  public boolean validate(Connection con) throws SQLException {
    Statement s = con.createStatement();
    ResultSet rs = s.executeQuery("SELECT column_name, data_type, character_maximum_length FROM INFORMATION_SCHEMA.COLUMNS where table_name = 'key_table'");

    int seen = 0;
    while(rs.next()) {
      seen++;
      String columnName = rs.getString(1);

      logger.log(Level.DEBUG,"Seen " + columnName + " with " + rs.getString(2) + " and " + rs.getInt(3));
      if (columnName.equals("key") || columnName.equals("secret")) {
        if (!rs.getString(2).equals("character"))
          return false;
        if (rs.getInt(3) != 40)
          return false;
      } else {
        return false;
      }
    }

    logger.log(Level.DEBUG,"Seen " + seen + " columns");
    
    if (seen > 0)
      return true;
    else
      return false;
  }

  public Database getNextVersion() {
    return null;
  }
}
