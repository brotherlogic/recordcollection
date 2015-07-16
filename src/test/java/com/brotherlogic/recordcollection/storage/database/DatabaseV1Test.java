package com.brotherlogic.recordcollection.storage.database;

import com.brotherlogic.recordcollection.BaseTest;

import org.mockito.Mockito;

import org.junit.Assert;
import org.junit.Test;

import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;

public class DatabaseV1Test extends BaseTest {

  @Test
  public void testCreate() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    DatabaseV1 db = new DatabaseV1();
    db.create(mConnection);

    Mockito.verify(mStatement).execute(Mockito.any(String.class));
  }

  @Test
  public void testUpgrade() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    DatabaseV1 db = new DatabaseV1();
    db.upgrade(mConnection);

    Mockito.verify(mStatement).execute(Mockito.any(String.class));
  }


  @Test
  public void testDestroy() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    DatabaseV1 db = new DatabaseV1();
    db.destroy(mConnection);

    Mockito.verify(mStatement).execute(Mockito.any(String.class));
  }

  @Test
  public void testGetNext() {
    DatabaseV1 db = new DatabaseV1();
    Assert.assertNull(db.getNextVersion());
  }

  @Test
  public void testValidateFailOnRandoCol() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","madeup");
    Mockito.when(mResultSet.getString(2)).thenReturn("character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV1 db = new DatabaseV1();
    Assert.assertFalse(db.validate(mConnection));
  }

  @Test
  public void testValidateFailOnBadColSize() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret");
    Mockito.when(mResultSet.getString(2)).thenReturn("character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(30);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV1 db = new DatabaseV1();
    Assert.assertFalse(db.validate(mConnection));
  }

  @Test
  public void testValidateFailOnBadColType() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret");
    Mockito.when(mResultSet.getString(2)).thenReturn("integer");
    Mockito.when(mResultSet.getInt(3)).thenReturn(20);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV1 db = new DatabaseV1();
    Assert.assertFalse(db.validate(mConnection));
  }
  
  @Test
  public void testValidate() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Statement mStatement = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mStatement);

    ResultSet mResultSet = Mockito.mock(ResultSet.class);
    Mockito.when(mResultSet.next()).thenReturn(true,true,false);

    Mockito.when(mResultSet.getString(1)).thenReturn("key","secret");
    Mockito.when(mResultSet.getString(2)).thenReturn("character");
    Mockito.when(mResultSet.getInt(3)).thenReturn(40);

    Mockito.when(mStatement.executeQuery(Mockito.any(String.class))).thenReturn(mResultSet);
    
    DatabaseV1 db = new DatabaseV1();
    Assert.assertTrue(db.validate(mConnection));
  }
}
