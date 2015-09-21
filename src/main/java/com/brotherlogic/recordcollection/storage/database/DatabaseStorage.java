package com.brotherlogic.recordcollection.storage.database;

import com.brotherlogic.recordcollection.storage.Storage;
import com.brotherlogic.recordcollection.RecordCollection;
import com.brotherlogic.recordcollection.RequestBuilder;
import com.brotherlogic.recordcollection.ScribeRetriever;

import java.util.Collections;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.Statement;
import java.sql.SQLException;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.scribe.oauth.OAuthService;
import org.scribe.model.Token;

public class DatabaseStorage implements Storage {

  private Connection connection;
  private PreparedStatement getTokenStmt;
  private PreparedStatement storeTokenStmt;
  private PreparedStatement storeCollectionStmt;
  private PreparedStatement getCollectionStmt;
  private PreparedStatement getCollectionsStmt;
  private Logger logger = Logger.getLogger(getClass());
  
  public DatabaseStorage(Connection conn) throws SQLException {
    connection = conn;
    getTokenStmt = connection.prepareStatement("SELECT secret FROM key_table where key = ?");
    storeTokenStmt = connection.prepareStatement("INSERT INTO key_table (key,secret) VALUES (?,?)");
    storeCollectionStmt = connection.prepareStatement("INSERT INTO collection (userid,name,folders,proportions) VALUES (?,?,?,?)");
    getCollectionStmt = connection.prepareStatement("SELECT folders,proportions from collection where userid = ? AND name = ?");
    getCollectionsStmt = connection.prepareStatement("SELECT folders, proportions, name from collection where userid = ?");
  }

  protected String convertList(List<Integer> vals) {
    if (vals.size() == 0)
      return "";
    
    Collections.sort(vals);
    String ret = vals.get(0).toString();

    for(int i = 1; i < vals.size() ; i++) {
      ret += "," + vals.get(i);
    }

    return ret;
  }

  protected List<Integer> convertString(String listStr) {
    logger.log(Level.INFO,"Converting List: " + listStr);
    List<Integer> ints = new LinkedList<Integer>();
    for(String elem : listStr.split(",")) {
      logger.log(Level.INFO,"Converted to " + elem + ":");
      ints.add(Integer.parseInt(elem));
    }
    logger.log(Level.INFO,"LIST CONVERSION: " + ints);
    return ints;
  }

  @Override
  public void storeCollection(Integer userId, RecordCollection col) {
    try{
      storeCollectionStmt.setString(3, convertList(col.getFolders()));
      storeCollectionStmt.setString(4, convertList(col.getProps()));
      storeCollectionStmt.setString(2, col.getName());
      storeCollectionStmt.setInt(1, userId);

      logger.log(Level.INFO,"Storing Collection: " + storeCollectionStmt);
      
      storeCollectionStmt.execute();
    } catch (SQLException e) {
      logger.log(Level.ERROR,"Cannot store collection",e);
    }
  }

  @Override
  public RecordCollection getCollection(Integer userId, String name) {
    try {
      return getCollection(userId, name, getCollectionStmt);
    } catch (SQLException e) {
      logger.log(Level.ERROR, "Failed to get Collection", e);
    }

    return null;
  }

  protected RecordCollection getCollection(Integer userId, String name, PreparedStatement s) throws SQLException {
    s.setInt(1,userId);
    s.setString(2,name);

    logger.log(Level.INFO,"Statement: " + s + " with " + userId + " and " + name);
    ResultSet rs = s.executeQuery();
    if (rs.next()) {
      logger.log(Level.INFO,"Found Response " + rs);
      return new RecordCollection(convertString(rs.getString(1).trim()), convertString(rs.getString(2).trim()), name);
    }

    return null;
  }

  @Override
  public List<RecordCollection> getCollections(Integer userId) {
    try {
      return getCollections(userId, getCollectionsStmt);
    } catch (SQLException e) {
      logger.log(Level.INFO,"Failed to run get Collections");
    }
    
    return null;
  }

  protected List<RecordCollection> getCollections(Integer userId, PreparedStatement s) throws SQLException {
    List<RecordCollection> collections = new LinkedList<RecordCollection>();
    
    s.setInt(1,userId);

    logger.log(Level.INFO,"Running " + s);
    ResultSet rs = s.executeQuery();
    while(rs.next()) {
      logger.log(Level.INFO,"Found response " + rs);
      RecordCollection r = new RecordCollection(convertString(rs.getString(1).trim()), convertString(rs.getString(2).trim()), rs.getString(3));
      collections.add(r);
    }

    return collections;
  }
    
  @Override
  public void storeToken(Token tok) {
    try {
      storeTokenStmt.setString(1,tok.getToken());
      storeTokenStmt.setString(2,tok.getSecret());
      storeTokenStmt.execute();
    } catch (SQLException e) {
      logger.log(Level.ERROR,"Cannot store token",e);
    }
  }
  
  @Override
  public Token getToken(String userKey) {
    try {
      return getToken(userKey, getTokenStmt);
    } catch (SQLException e) {
      logger.log(Level.ERROR,"Failed to run get token",e);
    }

    return null;
  }

  public void forceCleanDatabase() {
    try{
      List<String> tableNames = new LinkedList<String>();
      
      Statement s = connection.createStatement();
      ResultSet rs = s.executeQuery("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='public'");
      while(rs.next()) {
        tableNames.add(rs.getString(1));
      }

      logger.log(Level.INFO,"FOUND Tables: " + tableNames);
      
      for(String table : tableNames) {
        logger.log(Level.INFO,"Dropping Table " + table);
        connection.createStatement().execute("DROP TABLE " + table);
      }
    } catch (SQLException e) {
      logger.log(Level.ERROR,"Cannot clean database",e);
    }
  }

  
  protected Token getToken(String userKey, PreparedStatement s) throws SQLException {
    s.setString(1,userKey);
    ResultSet rs = s.executeQuery();

    if (rs.next()) {
      String secret = rs.getString(1).trim();      
      Token tok = new Token(userKey,secret);
      return tok;
    }

    return null;
  }
}
