package com.brotherlogic.recordcollection.storage.database;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.LinkedList;
import java.util.List;
import java.util.Stack;

public class DatabaseSystem {

  private Database db;
  private Logger logger = Logger.getLogger(getClass());
  
  public DatabaseSystem(Database init) {
    db = init;
  }
  
  public void initDatabase(Connection con) throws SQLException {
    Database current = db;
    Stack<Database> toUpgrade = new Stack<Database>();
    while (current != null && !current.validate(con)) {
      toUpgrade.push(current);
      current = current.getPrevVersion();
    }

    for(Database d : toUpgrade) {
      d.upgrade(con);
    }
  }
  
  public void cleanDatabase(Connection con) throws SQLException {
    boolean clean = false;
    Database curr = db;

    while(curr != null) {
      if (curr.validate(con)) {
        curr.destroy(con);
      }
      curr = curr.getPrevVersion();
    }
  }
}
