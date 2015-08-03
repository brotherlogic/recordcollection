package com.brotherlogic.recordcollection.storage.database;

import com.brotherlogic.recordcollection.BaseTest;

import org.mockito.Mockito;

import org.junit.Assert;
import org.junit.Test;

import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;

public class DatabaseV2Test extends BaseTest {

  @Test
  public void testCreate() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    DatabaseV2 db = new DatabaseV2();
    db.create(mConnection);

    Mockito.verify(mStatement,Mockito.times(2)).execute(Mockito.any(String.class));
  }

  @Test
  public void testUpgrade() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    DatabaseV2 db = new DatabaseV2();
    db.upgrade(mConnection);

    Mockito.verify(mStatement).execute(Mockito.any(String.class));
  }


  @Test
  public void testDestroy() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    DatabaseV2 db = new DatabaseV2();
    db.destroy(mConnection);

    Mockito.verify(mStatement, Mockito.times(2)).execute(Mockito.any(String.class));
  }

  @Test
  public void testGetPrev() {
    DatabaseV2 db = new DatabaseV2();
    Assert.assertEquals(DatabaseV1.class,db.getPrevVersion().getClass());
  }

  @Test
  public void testValidateFailOnRandoCol() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,true);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","madeup");
    Mockito.when(mResultSet.getString(2)).thenReturn("character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertFalse(db.validate(mConnection));
  }

  @Test
  public void testValidateFailOnBadParent() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,true);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","name");
    Mockito.when(mResultSet.getString(2)).thenReturn("integer");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40,40,40);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertFalse(db.validate(mConnection));
  }
  
  @Test
  public void testValidateFailOnBadColSize() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,true);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","name");
    Mockito.when(mResultSet.getString(2)).thenReturn("character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40,40,40);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertFalse(db.validate(mConnection));
  }

  @Test
  public void testValidateFailOnBadColType() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,true);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","name");
    Mockito.when(mResultSet.getString(2)).thenReturn("character","character","integer");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertFalse(db.validate(mConnection));
  }

  @Test
  public void testValidateBadInteger() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,true,true,true,true,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","name","folders", "proportions","userid");
    Mockito.when(mResultSet.getString(2)).thenReturn("character","character","character","character","character","character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40,40,60,60,60);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertFalse(db.validate(mConnection));
  }

  @Test
  public void testValidateNoSecond() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","name","folders", "proportions","userid");
    Mockito.when(mResultSet.getString(2)).thenReturn("character","character","character","character","character","character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40,40,60,60,60);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertFalse(db.validate(mConnection));
  }

  
  @Test
  public void testValidate() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false,true,true,true,true,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret","name","folders", "proportions","userid");
    Mockito.when(mResultSet.getString(2)).thenReturn("character","character","character","character","character","integer");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40,40,60,60,60);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV2 db = new DatabaseV2();
    Assert.assertTrue(db.validate(mConnection));
  }
}
