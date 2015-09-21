package com.brotherlogic.recordcollection;

import java.io.IOException;
import java.util.Map;
import java.util.TreeMap;

import javax.servlet.ServletContextEvent;
import javax.servlet.ServletContextListener;

import org.apache.log4j.ConsoleAppender;
import org.apache.log4j.FileAppender;
import org.apache.log4j.Level;
import org.apache.log4j.Logger;
import org.apache.log4j.PatternLayout;

import org.scribe.builder.ServiceBuilder;
import org.scribe.oauth.OAuthService;
import org.scribe.model.Token;

public class ContextListener implements ServletContextListener {
  
  private final String CALLBACK_URL = "http://localhost";

  public static final int TESTING = 0;
  @Override
  public void contextDestroyed(ServletContextEvent arg0) {

  }
  
  @Override
  public void contextInitialized(ServletContextEvent arg0) {
    //Setup the logging environment
    ConsoleAppender console = new ConsoleAppender();
    String pattern = "[%c] [%d{dd MMM yyyy - hh:mm:ss}] %5p - %m %n";
    console.setLayout(new PatternLayout(pattern));
    console.setThreshold(Level.INFO);
    console.activateOptions();
    Logger.getRootLogger().removeAllAppenders();
    Logger.getRootLogger().addAppender(console);

    //    try {
    //  Logger.getRootLogger().addAppender(new FileAppender(new PatternLayout(),"server.log"));
    //} catch (IOException e){
    //  e.printStackTrace();
    //}
    
    arg0.getServletContext().setAttribute("system", new MainSystem());
    arg0.getServletContext().setAttribute("token_map", new TreeMap<String,Token>());
  }
}
